package transport

import (
	"context"
	quic "github.com/lucas-clemente/quic-go"
	"net"
)

type QUIC struct {
	session quic.Session
}

func NewQUIC(serverSession quic.Session) *QUIC {
	return &QUIC{
		session: serverSession,
	}
}

func (qu *QUIC) Server() error {
	return nil
}

func (qu *QUIC) Accept() (net.Conn, error) {
	s, err := qu.session.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	return NewQUICConn(s, qu.session.RemoteAddr(), qu.session.LocalAddr()), nil
}

func (qu *QUIC) Addr() net.Addr {
	return qu.session.LocalAddr()
}

func (qu *QUIC) RemoteAddr() net.Addr {
	return qu.session.RemoteAddr()
}

func (qu *QUIC) Client() error {
	return nil
}

func (qu *QUIC) Open() (net.Conn, error) {
	s, err := qu.session.OpenStream()
	if err != nil {
		return nil, err
	}
	return NewQUICConn(s, qu.session.RemoteAddr(), qu.session.LocalAddr()), nil
}

func (qu *QUIC) Close() error {
	return qu.session.CloseWithError(1, "by npc")
}

type QUICConn struct {
	quic.Stream
	localAddr  net.Addr
	remoteAddr net.Addr
}

func NewQUICConn(stream quic.Stream, rd net.Addr, ld net.Addr) *QUICConn {
	return &QUICConn{Stream: stream, localAddr: ld, remoteAddr: rd}
}

func (qc *QUICConn) LocalAddr() net.Addr {
	return qc.localAddr
}

func (qc *QUICConn) RemoteAddr() net.Addr {
	return qc.remoteAddr
}
