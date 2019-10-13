package mux

import (
	"sync"
	"time"
)

type connMap struct {
	connMap map[int32]*conn
	closeCh chan struct{}
	sync.RWMutex
}

func NewConnMap() *connMap {
	connMap := &connMap{
		connMap: make(map[int32]*conn),
		closeCh: make(chan struct{}),
	}
	go connMap.clean()
	return connMap
}

func (s *connMap) Size() (n int) {
	return len(s.connMap)
}

func (s *connMap) Get(id int32) (*conn, bool) {
	s.Lock()
	defer s.Unlock()
	if v, ok := s.connMap[id]; ok && v != nil {
		return v, true
	}
	return nil, false
}

func (s *connMap) Set(id int32, v *conn) {
	s.Lock()
	defer s.Unlock()
	s.connMap[id] = v
}

func (s *connMap) Close() {
	s.Lock()
	defer s.Unlock()
	for _, v := range s.connMap {
		v.isClose = true
	}
	s.closeCh <- struct{}{}
}

func (s *connMap) Delete(id int32) {
	s.Lock()
	defer s.Unlock()
	delete(s.connMap, id)
}

func (s *connMap) clean() {
	ticker := time.NewTimer(time.Minute * 1)
	for {
		select {
		case <-ticker.C:
			s.Lock()
			for _, v := range s.connMap {
				if v.isClose {
					delete(s.connMap, v.connId)
				}
			}
			s.Unlock()
		case <-s.closeCh:
			ticker.Stop()
			return
		}
	}
}
