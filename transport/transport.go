package transport

import (
	"net"
)

type TunnelType int

type Conn interface {
	Server() error
	Accept() (net.Conn, error)
	Addr() net.Addr
	RemoteAddr() net.Addr
	Client() error
	Open() (net.Conn, error)
	Close() error
}
