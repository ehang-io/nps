package client

import (
	"crypto/tls"
	"ehang.io/nps/lib/cert"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/lib/pb"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
)

// StartNpc is used to connect to bridge
// proto is quic or tcp
// tlsConfig must contain a npc cert
func StartNpc(proto string, bridgeAddr string, tlsConfig *tls.Config) error {
	id, err := cert.GetCertSnFromConfig(tlsConfig)
	if err != nil {
		return err
	}
	var creator TunnelCreator
	if proto == "quic" {
		creator = QUICTunnelCreator{}
	} else {
		creator = TcpTunnelCreator{}
	}
	connId := uuid.NewV1().String()
retry:
	logger.Info("start connecting to bridge")
	controlLn, err := creator.NewMux(bridgeAddr,
		&pb.ConnRequest{Id: id, ConnType: &pb.ConnRequest_NpcInfo{NpcInfo: &pb.NpcInfo{TunnelId: connId, IsControlTunnel: true}}}, tlsConfig)
	if err != nil {
		logger.Error("new control connection error", zap.Error(err))
		goto retry
	}
	dataLn, err := creator.NewMux(bridgeAddr,
		&pb.ConnRequest{Id: id, ConnType: &pb.ConnRequest_NpcInfo{NpcInfo: &pb.NpcInfo{TunnelId: connId, IsControlTunnel: false}}}, tlsConfig)
	if err != nil {
		logger.Error("new data connection error", zap.Error(err))
		goto retry
	}
	c := NewClient(controlLn, dataLn)
	c.Run()
	goto retry
}
