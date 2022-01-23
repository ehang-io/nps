package handler

import (
	"bytes"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"go.uber.org/zap"
)

type P2PHandler struct {
	DefaultHandler
}

func (ph *P2PHandler) GetName() string {
	return "p2p"
}

func (ph *P2PHandler) GetZhName() string {
	return "点对点协议"
}

func (ph *P2PHandler) HandlePacketConn(pc enet.PacketConn) (bool, error) {
	b, _, err := pc.FirstPacket()
	if err != nil {
		logger.Warn("firstPacket error", zap.Error(err))
		return false, nil
	}
	if bytes.HasPrefix(b, []byte("p2p")) {
		return ph.processPacketConn(pc)
	}
	return false, nil
}
