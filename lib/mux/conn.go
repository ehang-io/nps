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
	readMsgCh        chan []byte
	getStatusCh      chan struct{}
	connStatusOkCh   chan struct{}
	connStatusFailCh chan struct{}
	readTimeOut      time.Time
	writeTimeOut     time.Time
	sendMsgCh        chan *msg  //mux
	sendStatusCh     chan int32 //mux
	connId           int32
	isClose          bool
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
		readMsgCh:        make(chan []byte),
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

func (s *conn) Read(buf []byte) (int, error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}
	var b []byte
	if t := s.readTimeOut.Sub(time.Now()); t > 0 {
		timer := time.NewTimer(t)
		select {
		case <-timer.C:
			s.Close()
			return 0, errors.New("read timeout")
		case b = <-s.readMsgCh:
		}
	} else {
		b = <-s.readMsgCh
	}
	defer pool.PutBufPoolCopy(b)
	if s.isClose {
		return 0, io.EOF
	}
	s.sendStatusCh <- s.connId
	return copy(buf, b), nil
}

func (s *conn) Write(buf []byte) (int, error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}

	if t := s.writeTimeOut.Sub(time.Now()); t > 0 {
		timer := time.NewTimer(t)
		select {
		case <-timer.C:
			s.Close()
			return 0, errors.New("write timeout")
		case s.sendMsgCh <- NewMsg(s.connId, buf):
		}
	} else {
		s.sendMsgCh <- NewMsg(s.connId, buf)
	}

	if t := s.writeTimeOut.Sub(time.Now()); t > 0 {
		timer := time.NewTimer(t)
		select {
		case <-timer.C:
			s.Close()
			return 0, errors.New("write timeout")
		case <-s.getStatusCh:
		}
	} else {
		<-s.getStatusCh
	}

	if s.isClose {
		return 0, io.EOF
	}
	return len(buf), nil
}

func (s *conn) Close() error {
	if s.isClose {
		return errors.New("the conn has closed")
	}
	s.isClose = true
	close(s.getStatusCh)
	close(s.readMsgCh)
	close(s.connStatusOkCh)
	close(s.connStatusFailCh)
	s.sendMsgCh <- NewMsg(s.connId, nil)
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
