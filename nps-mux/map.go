package nps_mux

import (
	"sync"
)

type connMap struct {
	cMap map[int32]*conn
	//closeCh chan struct{}
	sync.RWMutex
}

func NewConnMap() *connMap {
	cMap := &connMap{
		cMap: make(map[int32]*conn),
	}
	return cMap
}

func (s *connMap) Size() (n int) {
	s.RLock()
	n = len(s.cMap)
	s.RUnlock()
	return
}

func (s *connMap) Get(id int32) (*conn, bool) {
	s.RLock()
	v, ok := s.cMap[id]
	s.RUnlock()
	if ok && v != nil {
		return v, true
	}
	return nil, false
}

func (s *connMap) Set(id int32, v *conn) {
	s.Lock()
	s.cMap[id] = v
	s.Unlock()
}

func (s *connMap) Close() {
	for _, v := range s.cMap {
		_ = v.Close() // close all the connections in the mux
	}
}

func (s *connMap) Delete(id int32) {
	s.Lock()
	delete(s.cMap, id)
	s.Unlock()
}
