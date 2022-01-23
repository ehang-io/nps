package lb

import (
	"errors"
	"sync"
)

func GetLbAlgo(algo string) Algo {
	// switch
	return NewRoundRobin()
}

type Algo interface {
	Next() (interface{}, error)
	Append(i interface{}) error
	Remove(i interface{}) error
	Empty() bool
}

// rotation
type roundRobin struct {
	head *server
	now  *server
	sync.RWMutex
}

type server struct {
	self interface{}
	next *server
}

func NewRoundRobin() *roundRobin {
	return &roundRobin{}
}

func (r *roundRobin) Append(i interface{}) error {
	r.Lock()
	defer r.Unlock()
	if r.head == nil {
		r.head = &server{self: i}
		return nil
	}
	r.now = r.head
	for {
		if r.now.next == nil {
			r.now.next = &server{self: i}
			break
		}
		r.now = r.now.next
	}
	return nil
}

func (r *roundRobin) Remove(i interface{}) error {
	r.Lock()
	defer r.Unlock()
	o := r.head
	var last *server
	for {
		if o == nil {
			return errors.New("not round")
		}
		if o.self == i {
			if last == nil {
				r.head = o.next
			} else {
				last.next = o.next
			}
			r.now = r.head
			return nil
		}
		last = o
		o = o.next
	}
}

func (r *roundRobin) Next() (interface{}, error) {
	r.Lock()
	defer r.Unlock()
	if r.head == nil {
		return nil, errors.New("not found component")
	}
	if r.now == nil {
		r.now = r.head
	}
	i := r.now
	r.now = r.now.next
	return i.self, nil
}

func (r *roundRobin) Empty() bool {
	if r.head != nil {
		return false
	}
	return true
}
