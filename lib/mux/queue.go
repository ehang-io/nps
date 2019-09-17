package mux

import (
	"container/list"
	"github.com/cnlh/nps/lib/common"
	"sync"
)

type Queue struct {
	list    *list.List
	readOp  chan struct{}
	cleanOp chan struct{}
	popWait bool
	mutex   sync.Mutex
}

func (Self *Queue) New() {
	Self.list = list.New()
	Self.readOp = make(chan struct{})
	Self.cleanOp = make(chan struct{}, 2)
}

func (Self *Queue) Push(packager *common.MuxPackager) {
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

func (Self *Queue) allowPop() (closed bool) {
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

func (Self *Queue) insert(packager *common.MuxPackager) {
	element := Self.list.Back()
	for {
		if element == nil { // Queue dose not have any of msg package with this close package id
			Self.list.PushFront(packager) // insert close package to first
			break
		}
		if element.Value.(*common.MuxPackager).Flag == common.MUX_NEW_MSG &&
			element.Value.(*common.MuxPackager).Id == packager.Id {
			Self.list.InsertAfter(packager, element) // Queue has some msg package
			// with this close package id, insert close package after last msg package
			break
		}
		element = element.Prev()
	}
}

func (Self *Queue) Pop() (packager *common.MuxPackager) {
	Self.mutex.Lock()
	element := Self.list.Front()
	if element != nil {
		packager = element.Value.(*common.MuxPackager)
		Self.list.Remove(element)
		Self.mutex.Unlock()
		return
	}
	Self.popWait = true // Queue is empty, notice Push method
	Self.mutex.Unlock()
	select {
	case <-Self.readOp:
		return Self.Pop()
	case <-Self.cleanOp:
		return nil
	}
}

func (Self *Queue) Len() (n int) {
	n = Self.list.Len()
	return
}

func (Self *Queue) Clean() {
	Self.cleanOp <- struct{}{}
	Self.cleanOp <- struct{}{}
	close(Self.cleanOp)
}
