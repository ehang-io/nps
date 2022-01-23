package enet

import (
	"errors"
	"net"
	"sync/atomic"
)

var _ net.Listener = (*Listener)(nil)

// Listener is an implementation of net.Listener
type Listener struct {
	ch      chan net.Conn
	closeCh chan struct{}
	closed  int32
	nowNum  int32
	addr    net.Addr
}

// NewListener returns an initialized Listener
func NewListener() *Listener {
	return &Listener{ch: make(chan net.Conn, 10), closeCh: make(chan struct{})}
}

// SendConn is used to add connection to the listener
func (bl *Listener) SendConn(c net.Conn) error {
	if atomic.LoadInt32(&bl.closed) == 1 {
		return errors.New("the listener is already closed")
	}
	atomic.AddInt32(&bl.nowNum, 1)
	select {
	case bl.ch <- c:
		return nil
	case <-bl.closeCh:
	}
	if atomic.AddInt32(&bl.nowNum, -1) == 0 && atomic.LoadInt32(&bl.closed) == 1 {
		close(bl.ch)
	}
	return errors.New("the listener is already closed")
}

// Accept is used to get connection from the listener
func (bl *Listener) Accept() (net.Conn, error) {
	c := <-bl.ch
	if c == nil {
		return nil, errors.New("the listener is already closed")
	}
	return c, nil
}

// Close is used to close the listener, it will discard all  existing connections
func (bl *Listener) Close() error {
	if atomic.CompareAndSwapInt32(&bl.closed, 0, 1) {
		close(bl.closeCh)
	}
	return nil
}

// Addr returns the listener's address'
func (bl *Listener) Addr() net.Addr {
	return bl.addr
}
