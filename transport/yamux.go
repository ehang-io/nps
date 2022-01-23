package transport

import (
	"github.com/hashicorp/yamux"
	"net"
)

type YaMux struct {
	conn    net.Conn
	config  *yamux.Config
	session *yamux.Session
}

func NewYaMux(conn net.Conn, config *yamux.Config) *YaMux {
	return &YaMux{
		conn:   conn,
		config: config,
	}
}

func (ym *YaMux) Server() error {
	var err error
	ym.session, err = yamux.Server(ym.conn, ym.config)
	return err
}

func (ym *YaMux) Accept() (net.Conn, error) {
	return ym.session.Accept()
}

func (ym *YaMux) Addr() net.Addr {
	return ym.conn.LocalAddr()
}

func (ym *YaMux) RemoteAddr() net.Addr {
	return ym.conn.RemoteAddr()
}

func (ym *YaMux) Client() error {
	var err error
	ym.session, err = yamux.Client(ym.conn, ym.config)
	return err
}

func (ym *YaMux) Open() (net.Conn, error) {
	return ym.session.Open()
}

func (ym *YaMux) Close() error {
	return ym.conn.Close()
}
