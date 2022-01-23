package bridge

import (
	"context"
	"crypto/tls"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/lib/pb"
	"ehang.io/nps/transport"
	"github.com/lucas-clemente/quic-go"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"io"
	"net"
)

type QUICServer struct {
	packetConn  net.PacketConn
	tlsConfig   *tls.Config
	config      *quic.Config
	listener    quic.Listener
	gp          *ants.PoolWithFunc
	clientCheck func(string) bool
	manager     *manager
}

func NewQUICServer(packetConn net.PacketConn, tlsConfig *tls.Config, config *quic.Config, clientCheck func(string) bool) (*QUICServer, error) {
	qs := &QUICServer{
		packetConn:  packetConn,
		tlsConfig:   tlsConfig,
		config:      config,
		clientCheck: clientCheck,
		manager:     NewManager(),
	}
	var err error
	if qs.listener, err = quic.Listen(packetConn, tlsConfig, config); err != nil {
		return nil, err
	}
	qs.gp, err = ants.NewPoolWithFunc(1000000, func(i interface{}) {
		session := i.(quic.Session)
		logger.Debug("accept a session", zap.String("remote addr", session.RemoteAddr().String()))
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			logger.Warn("accept stream error", zap.Error(err))
			_ = session.CloseWithError(0, "check auth failed")
			return
		}
		cr := &pb.ConnRequest{}
		_, err = pb.ReadMessage(stream, cr)
		if err != nil {
			_ = session.CloseWithError(0, "check auth failed")
			logger.Warn("read message error", zap.Error(err))
			return
		}
		if !qs.clientCheck(cr.GetId()) {
			_ = session.CloseWithError(0, "check auth failed")
			logger.Error("check server id error", zap.String("id", cr.GetId()))
			_ = qs.responseClient(stream, false, "id check failed")
			return
		}
		qc := transport.NewQUIC(session)
		_ = qc.Client()

		_ = qs.responseClient(stream, true, "success")
		err = qs.manager.SetClient(cr.GetId(), cr.GetNpcInfo().GetTunnelId(), cr.GetNpcInfo().GetIsControlTunnel(), qc)
		if err != nil {
			_ = session.CloseWithError(0, "check auth failed")
			logger.Error("set client error", zap.Error(err), zap.String("info", cr.String()))
		}
	})
	return qs, err
}

func (qs *QUICServer) responseClient(conn io.Writer, success bool, msg string) error {
	_, err := pb.WriteMessage(conn, &pb.NpcResponse{Success: success, Message: msg})
	return err
}

func (qs *QUICServer) run() error {
	for {
		session, err := qs.listener.Accept(context.Background())
		if err != nil {
			logger.Error("accept connection failed", zap.Error(err))
			return err
		}
		err = qs.gp.Invoke(session)
		if err != nil {
			logger.Error("Invoke session error", zap.Error(err))
			continue
		}
	}
}
