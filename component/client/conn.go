package client

import (
	"crypto/tls"
	"ehang.io/nps/lib/pb"
	"ehang.io/nps/transport"
	"github.com/lucas-clemente/quic-go"
	"github.com/pkg/errors"
	"io"
	"net"
	"time"
)

type TunnelCreator interface {
	NewMux(bridgeAddr string, message *pb.ConnRequest, config *tls.Config) (net.Listener, error)
}

type BaseTunnelCreator struct{}

func (bc BaseTunnelCreator) handshake(npcInfo *pb.ConnRequest, rw io.ReadWriteCloser) error {
	_, err := pb.WriteMessage(rw, npcInfo)
	if err != nil {
		return errors.Wrap(err, "write handshake message")
	}
	var resp pb.NpcResponse
	_, err = pb.ReadMessage(rw, &resp)
	if err != nil || !resp.Success {
		return errors.Wrap(err, resp.Message)
	}
	return nil
}

type TcpTunnelCreator struct{ BaseTunnelCreator }

func (tc TcpTunnelCreator) NewMux(bridgeAddr string, message *pb.ConnRequest, config *tls.Config) (net.Listener, error) {
	conn, err := tls.Dial("tcp", bridgeAddr, config)
	if err != nil {
		return nil, err
	}
	if err := tc.handshake(message, conn); err != nil {
		return nil, err
	}
	server := transport.NewYaMux(conn, nil)
	return server, server.Server()
}

type QUICTunnelCreator struct{ BaseTunnelCreator }

func (tc QUICTunnelCreator) NewMux(bridgeAddr string, message *pb.ConnRequest, config *tls.Config) (net.Listener, error) {
	session, err := quic.DialAddr(bridgeAddr, config, &quic.Config{
		MaxIncomingStreams:    1000000,
		MaxIncomingUniStreams: 1000000,
		MaxIdleTimeout:        time.Minute,
		KeepAlive:             true,
	})
	if err != nil {
		return nil, err
	}
	stream, err := session.OpenStream()
	if err != nil {
		return nil, err
	}
	err = tc.handshake(message, stream)
	if err != nil {
		return nil, err
	}
	server := transport.NewQUIC(session)
	return server, server.Server()
}
