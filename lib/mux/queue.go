package mux

import (
	"container/list"
	"errors"
	"github.com/cnlh/nps/lib/common"
	"io"
	"sync"
	"time"
)

type QueueOp struct {
	readOp  chan struct{}
	cleanOp chan struct{}
	popWait bool
	mutex   sync.Mutex
}

func (Self *QueueOp) New() {
	Self.readOp = make(chan struct{})
	Self.cleanOp = make(chan struct{}, 2)
}

func (Self *QueueOp) allowPop() (closed bool) {
	Self.mutex.Lock()
	Self.popWait = false
	Self.mutex.Unlock()
	select {
	case Self.readOp <- struct{}{}:
		return false
	case <-Self.cleanOp:
		return true
	}
}

func (Self *QueueOp) Clean() {
	Self.cleanOp <- struct{}{}
	Self.cleanOp <- struct{}{}
	close(Self.cleanOp)
}

type PriorityQueue struct {
	list *list.List
	QueueOp
}

func (Self *PriorityQueue) New() {
	Self.list = list.New()
	Self.QueueOp.New()
}

func (Self *PriorityQueue) Push(packager *common.MuxPackager) {
	Self.mutex.Lock()
	if Self.popWait {
		defer Self.allowPop()
	}
	if packager.Flag == common.MUX_CONN_CLOSE {
		Self.insert(packager) // the close package may need priority,
		// prevent wait too long to close
	} else {
		Self.list.PushBack(packager)
	}
	Self.mutex.Unlock()
	return
}

func (Self *PriorityQueue) insert(packager *common.MuxPackager) {
	element := Self.list.Back()
	for {
		if element == nil { // PriorityQueue dose not have any of msg package with this close package id
			Self.list.PushFront(packager) // insert close package to first
			break
		}
		if element.Value.(*common.MuxPackager).Flag == common.MUX_NEW_MSG &&
			element.Value.(*common.MuxPackager).Id == packager.Id {
			Self.list.InsertAfter(packager, element) // PriorityQueue has some msg package
			// with this close package id, insert close package after last msg package
			break
		}
		element = element.Prev()
	}
}

func (Self *PriorityQueue) Pop() (packager *common.MuxPackager) {
	Self.mutex.Lock()
	element := Self.list.Front()
	if element != nil {
		packager = element.Value.(*common.MuxPackager)
		Self.list.Remove(element)
		Self.mutex.Unlock()
		return
	}
	Self.popWait = true // PriorityQueue is empty, notice Push method
	Self.mutex.Unlock()
	select {
	case <-Self.readOp:
		return Self.Pop()
	case <-Self.cleanOp:
		return nil
	}
}

func (Self *PriorityQueue) Len() (n int) {
	n = Self.list.Len()
	return
}

type ListElement struct {
	buf  []byte
	l    uint16
	part bool
}

func (Self *ListElement) New(buf []byte, l uint16, part bool) (err error) {
	if uint16(len(buf)) != l {
		return errors.New("ListElement: buf length not match")
	}
	Self.buf = buf
	Self.l = l
	Self.part = part
	return nil
}

type FIFOQueue struct {
	list    []*ListElement
	length  uint32
	stopOp  chan struct{}
	timeout time.Time
	QueueOp
}

func (Self *FIFOQueue) New() {
	Self.QueueOp.New()
	Self.stopOp = make(chan struct{}, 1)
}

func (Self *FIFOQueue) Push(element *ListElement) {
	Self.mutex.Lock()
	if Self.popWait {
		defer Self.allowPop()
	}
	Self.list = append(Self.list, element)
	Self.length += uint32(element.l)
	Self.mutex.Unlock()
	return
}

func (Self *FIFOQueue) Pop() (element *ListElement, err error) {
	Self.mutex.Lock()
	if len(Self.list) == 0 {
		Self.popWait = true
		Self.mutex.Unlock()
		t := Self.timeout.Sub(time.Now())
		if t <= 0 {
			t = time.Minute
		}
		timer := time.NewTimer(t)
		defer timer.Stop()
		select {
		case <-Self.readOp:
			Self.mutex.Lock()
		case <-Self.cleanOp:
			return
		case <-Self.stopOp:
			err = io.EOF
			return
		case <-timer.C:
			err = errors.New("mux.queue: read time out")
			return
		}
	}
	element = Self.list[0]
	Self.list = Self.list[1:]
	Self.length -= uint32(element.l)
	Self.mutex.Unlock()
	return
}

func (Self *FIFOQueue) Len() (n uint32) {
	return Self.length
}

func (Self *FIFOQueue) Stop() {
	Self.stopOp <- struct{}{}
}

func (Self *FIFOQueue) SetTimeOut(t time.Time) {
	Self.timeout = t
}
