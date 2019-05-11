package mux

import (
	"errors"
	"github.com/cnlh/nps/lib/pool"
	"io"
	"net"
	"sync"
	"time"
)

type conn struct {
	net.Conn
	getStatusCh      chan struct{}
	connStatusOkCh   chan struct{}
	connStatusFailCh chan struct{}
	readTimeOut      time.Time
	writeTimeOut     time.Time
	readBuffer       []byte
	startRead        int //now read position
	endRead          int //now end read
	readFlag         bool
	readCh           chan struct{}
	waitQueue        *sliceEntry
	stopWrite        bool
	connId           int32
	isClose          bool
	readWait         bool
	hasWrite         int
	mux              *Mux
}

var connPool = sync.Pool{}

func NewConn(connId int32, mux *Mux) *conn {
	c := &conn{
		readCh:           make(chan struct{}),
		getStatusCh:      make(chan struct{}),
		connStatusOkCh:   make(chan struct{}),
		connStatusFailCh: make(chan struct{}),
		waitQueue:        NewQueue(),
		connId:           connId,
		mux:              mux,
	}
	return c
}

func (s *conn) Read(buf []byte) (n int, err error) {
	if s.isClose || buf == nil {
		return 0, errors.New("the conn has closed")
	}
	if s.endRead-s.startRead == 0 { //read finish or start
		if s.waitQueue.Size() == 0 {
			s.readWait = true
			if t := s.readTimeOut.Sub(time.Now()); t > 0 {
				timer := time.NewTimer(t)
				defer timer.Stop()
				select {
				case <-timer.C:
					s.readWait = false
					return 0, errors.New("read timeout")
				case <-s.readCh:
				}
			} else {
				<-s.readCh
			}
		}
		if s.isClose { //If the connection is closed instead of  continuing command
			return 0, errors.New("the conn has closed")
		}
		if node, err := s.waitQueue.Pop(); err != nil {
			s.Close()
			return 0, io.EOF
		} else {
			pool.PutBufPoolCopy(s.readBuffer)
			s.readBuffer = node.val
			s.endRead = node.l
			s.startRead = 0
		}
	}
	if len(buf) < s.endRead-s.startRead {
		n = copy(buf, s.readBuffer[s.startRead:s.startRead+len(buf)])
		s.startRead += n
	} else {
		n = copy(buf, s.readBuffer[s.startRead:s.endRead])
		s.startRead += n
		s.mux.sendInfo(MUX_MSG_SEND_OK, s.connId, nil)
	}
	return
}

func (s *conn) Write(buf []byte) (int, error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}
	ch := make(chan struct{})
	go s.write(buf, ch)
	if t := s.writeTimeOut.Sub(time.Now()); t > 0 {
		timer := time.NewTimer(t)
		defer timer.Stop()
		select {
		case <-timer.C:
			return 0, errors.New("write timeout")
		case <-ch:
		}
	} else {
		<-ch
	}
	if s.isClose {
		return 0, io.EOF
	}
	return len(buf), nil
}
func (s *conn) write(buf []byte, ch chan struct{}) {
	start := 0
	l := len(buf)
	for {
		if s.hasWrite > 50 {
			<-s.getStatusCh
		}
		s.hasWrite++
		if l-start > pool.PoolSizeCopy {
			s.mux.sendInfo(MUX_NEW_MSG, s.connId, buf[start:start+pool.PoolSizeCopy])
			start += pool.PoolSizeCopy
		} else {
			s.mux.sendInfo(MUX_NEW_MSG, s.connId, buf[start:l])
			break
		}
	}
	ch <- struct{}{}
}

func (s *conn) Close() error {
	if s.isClose {
		return errors.New("the conn has closed")
	}
	times := 0
retry:
	if s.waitQueue.Size() > 0 && times < 600 {
		time.Sleep(time.Millisecond * 100)
		times++
		goto retry
	}
	if s.isClose {
		return errors.New("the conn has closed")
	}
	s.isClose = true
	pool.PutBufPoolCopy(s.readBuffer)
	if s.readWait {
		s.readCh <- struct{}{}
	}
	s.waitQueue.Clear()
	s.mux.connMap.Delete(s.connId)
	if !s.mux.IsClose {
		s.mux.sendInfo(MUX_CONN_CLOSE, s.connId, nil)
	}
	connPool.Put(s)
	return nil
}

func (s *conn) LocalAddr() net.Addr {
	return s.mux.conn.LocalAddr()
}

func (s *conn) RemoteAddr() net.Addr {
	return s.mux.conn.RemoteAddr()
}

func (s *conn) SetDeadline(t time.Time) error {
	s.readTimeOut = t
	s.writeTimeOut = t
	return nil
}

func (s *conn) SetReadDeadline(t time.Time) error {
	s.readTimeOut = t
	return nil
}

func (s *conn) SetWriteDeadline(t time.Time) error {
	s.writeTimeOut = t
	return nil
}
