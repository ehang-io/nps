package pmux

import (
	"errors"
	"net"
)

type PortListener struct {
	net.Listener
	connCh  chan *PortConn
	addr    net.Addr
	isClose bool
}

func NewPortListener(connCh chan *PortConn, addr net.Addr) *PortListener {
	return &PortListener{
		connCh: connCh,
		addr:   addr,
	}
}

func (pListener *PortListener) Accept() (net.Conn, error) {
	if pListener.isClose {
		return nil, errors.New("the listener has closed")
	}
	conn := <-pListener.connCh
	if conn != nil {
		return conn, nil
	}
	return nil, errors.New("the listener has closed")
}

func (pListener *PortListener) Close() error {
	//close
	if pListener.isClose {
		return errors.New("the listener has closed")
	}
	pListener.isClose = true
	return nil
}

func (pListener *PortListener) Addr() net.Addr {
	return pListener.addr
}
