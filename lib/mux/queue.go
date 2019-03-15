package mux

import (
	"errors"
	"github.com/cnlh/nps/lib/pool"
	"sync"
)

type Element *bufNode

type bufNode struct {
	val []byte //buf value
	l   int    //length
}

func NewBufNode(buf []byte, l int) *bufNode {
	return &bufNode{
		val: buf,
		l:   l,
	}
}

type Queue interface {
	Push(e Element) //向队列中添加元素
	Pop() Element   //移除队列中最前面的元素
	Clear() bool    //清空队列
	Size() int      //获取队列的元素个数
	IsEmpty() bool  //判断队列是否是空
}

type sliceEntry struct {
	element []Element
	sync.Mutex
}

func NewQueue() *sliceEntry {
	return &sliceEntry{}
}

//向队列中添加元素
func (entry *sliceEntry) Push(e Element) {
	entry.Lock()
	defer entry.Unlock()
	entry.element = append(entry.element, e)
}

//移除队列中最前面的额元素
func (entry *sliceEntry) Pop() (Element, error) {
	if entry.IsEmpty() {
		return nil, errors.New("queue is empty!")
	}
	entry.Lock()
	defer entry.Unlock()
	firstElement := entry.element[0]
	entry.element = entry.element[1:]
	return firstElement, nil
}

func (entry *sliceEntry) Clear() bool {
	entry.Lock()
	defer entry.Unlock()
	if entry.IsEmpty() {
		return false
	}
	for i := 0; i < entry.Size(); i++ {
		pool.PutBufPoolCopy(entry.element[i].val)
		entry.element[i] = nil
	}
	entry.element = nil
	return true
}

func (entry *sliceEntry) Size() int {
	return len(entry.element)
}

func (entry *sliceEntry) IsEmpty() bool {
	if len(entry.element) == 0 {
		return true
	}
	return false
}
