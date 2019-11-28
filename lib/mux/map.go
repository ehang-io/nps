package mux

import (
	"sync"
)

type connMap struct {
	connMap map[int32]*conn
	//closeCh chan struct{}
	sync.RWMutex
}

func NewConnMap() *connMap {
	connMap := &connMap{
		connMap: make(map[int32]*conn),
		//closeCh: make(chan struct{}),
	}
	//go connMap.clean()
	return connMap
}

func (s *connMap) Size() (n int) {
	s.Lock()
	n = len(s.connMap)
	s.Unlock()
	return
}

func (s *connMap) Get(id int32) (*conn, bool) {
	s.Lock()
	v, ok := s.connMap[id]
	s.Unlock()
	if ok && v != nil {
		return v, true
	}
	return nil, false
}

func (s *connMap) Set(id int32, v *conn) {
	s.Lock()
	s.connMap[id] = v
	s.Unlock()
}

func (s *connMap) Close() {
	//s.closeCh <- struct{}{} // stop the clean goroutine first
	for _, v := range s.connMap {
		v.Close() // close all the connections in the mux
	}
}

func (s *connMap) Delete(id int32) {
	s.Lock()
	delete(s.connMap, id)
	s.Unlock()
}

//func (s *connMap) clean() {
//	ticker := time.NewTimer(time.Minute * 1)
//	for {
//		select {
//		case <-ticker.C:
//			s.Lock()
//			for _, v := range s.connMap {
//				if v.isClose {
//					delete(s.connMap, v.connId)
//				}
//			}
//			s.Unlock()
//		case <-s.closeCh:
//			ticker.Stop()
//			return
//		}
//	}
//}
