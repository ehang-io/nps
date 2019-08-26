package mux

import (
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"io"
	"net"
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
	readQueue        *sliceEntry
	stopWrite        bool
	connId           int32
	isClose          bool
	readWait         bool
	sendClose        bool
	hasWrite         int
	mux              *Mux
}

func NewConn(connId int32, mux *Mux) *conn {
	c := &conn{
		readCh:           make(chan struct{}),
		getStatusCh:      make(chan struct{}),
		connStatusOkCh:   make(chan struct{}),
		connStatusFailCh: make(chan struct{}),
		readQueue:        NewQueue(),
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
		if s.readQueue.Size() == 0 {
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
		if node, err := s.readQueue.Pop(); err != nil {
			s.Close()
			return 0, io.EOF
		} else {
			//pool.PutBufPoolCopy(s.readBuffer)
			if node.val == nil {
				//close
				s.sendClose = true
				s.Close()
				return 0, io.EOF
			} else {
				s.readBuffer = node.val
				s.endRead = node.l
				s.startRead = 0
			}
		}
	}
	if len(buf) < s.endRead-s.startRead {
		n = copy(buf, s.readBuffer[s.startRead:s.startRead+len(buf)])
		s.startRead += n
	} else {
		n = copy(buf, s.readBuffer[s.startRead:s.endRead])
		s.startRead += n
	}
	return
}

func (s *conn) Write(buf []byte) (n int, err error) {
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
		if l-start > pool.PoolSizeCopy {
			s.mux.sendInfo(common.MUX_NEW_MSG, s.connId, buf[start:start+pool.PoolSizeCopy])
			start += pool.PoolSizeCopy
		} else {
			s.mux.sendInfo(common.MUX_NEW_MSG, s.connId, buf[start:l])
			break
		}
	}
	ch <- struct{}{}
}

func (s *conn) Close() (err error) {
	if s.isClose {
		return errors.New("the conn has closed")
	}
	s.isClose = true
	pool.PutBufPoolCopy(s.readBuffer)
	if s.readWait {
		s.readCh <- struct{}{}
	}
	s.readQueue.Clear()
	s.mux.connMap.Delete(s.connId)
	if !s.mux.IsClose {
		if !s.sendClose {
			err = s.mux.sendInfo(common.MUX_CONN_CLOSE, s.connId, nil)
			logs.Warn("send closing msg ok ", s.connId)
			if err != nil {
				logs.Warn(err)
				return
			}
		} else {
		}
	}
	return
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
