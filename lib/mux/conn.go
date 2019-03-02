package mux

import (
	"errors"
	"github.com/cnlh/nps/lib/pool"
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
	sendMsgCh        chan *msg  //mux
	sendStatusCh     chan int32 //mux
	readBuffer       []byte
	startRead        int //now read position
	endRead          int //now end read
	readFlag         bool
	readCh           chan struct{}
	connId           int32
	isClose          bool
	readWait         bool
	mux              *Mux
}

type msg struct {
	connId  int32
	content []byte
}

func NewMsg(connId int32, content []byte) *msg {
	return &msg{
		connId:  connId,
		content: content,
	}
}

func NewConn(connId int32, mux *Mux, sendMsgCh chan *msg, sendStatusCh chan int32) *conn {
	return &conn{
		readCh:           make(chan struct{}),
		readBuffer:       pool.BufPoolCopy.Get().([]byte),
		getStatusCh:      make(chan struct{}),
		connStatusOkCh:   make(chan struct{}),
		connStatusFailCh: make(chan struct{}),
		readTimeOut:      time.Time{},
		writeTimeOut:     time.Time{},
		sendMsgCh:        sendMsgCh,
		sendStatusCh:     sendStatusCh,
		connId:           connId,
		isClose:          false,
		mux:              mux,
	}
}

func (s *conn) Read(buf []byte) (n int, err error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}
	if s.endRead-s.startRead == 0 {
		s.readWait = true
		if t := s.readTimeOut.Sub(time.Now()); t > 0 {
			timer := time.NewTimer(t)
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
	s.readWait = false
	if s.isClose {
		return 0, io.EOF
	}
	if len(buf) < s.endRead-s.startRead {
		n = copy(buf, s.readBuffer[s.startRead:s.startRead+len(buf)])
		s.startRead += n
	} else {
		n = copy(buf, s.readBuffer[s.startRead:s.endRead])
		s.startRead = 0
		s.endRead = 0
		s.sendStatusCh <- s.connId
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
			s.sendMsgCh <- NewMsg(s.connId, buf[start:start+pool.PoolSizeCopy])
			start += pool.PoolSizeCopy
			<-s.getStatusCh
		} else {
			s.sendMsgCh <- NewMsg(s.connId, buf[start:l])
			<-s.getStatusCh
			break
		}
	}
	ch <- struct{}{}
}

func (s *conn) Close() error {
	if s.isClose {
		return errors.New("the conn has closed")
	}
	s.isClose = true
	pool.PutBufPoolCopy(s.readBuffer)
	close(s.getStatusCh)
	close(s.connStatusOkCh)
	close(s.connStatusFailCh)
	close(s.readCh)
	if !s.mux.IsClose {
		s.sendMsgCh <- NewMsg(s.connId, nil)
	}
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
