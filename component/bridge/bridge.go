package bridge

import (
	"crypto/tls"
	"github.com/lucas-clemente/quic-go"
	"net"
)

func StartTcpBridge(ln net.Listener, config *tls.Config, serverCheck, clientCheck func(string) bool) error {
	h, err := NewTcpServer(ln, config, serverCheck, clientCheck)
	if err != nil {
		return err
	}
	return h.run()
}

func StartQUICBridge(ln net.PacketConn, config *tls.Config, quicConfig *quic.Config, clientCheck func(string) bool) error {
	h, err := NewQUICServer(ln, config, quicConfig, clientCheck)
	if err != nil {
		return err
	}
	return h.run()
}
