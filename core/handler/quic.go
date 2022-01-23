package handler

import (
	"bytes"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"go.uber.org/zap"
)

type QUICHandler struct {
	DefaultHandler
}

func (qh *QUICHandler) GetName() string {
	return "quic"
}

func (qh *QUICHandler) GetZhName() string {
	return "quic协议"
}

func (qh *QUICHandler) HandlePacketConn(pc enet.PacketConn) (bool, error) {
	b, _, err := pc.FirstPacket()
	if err != nil {
		logger.Warn("firstPacket error", zap.Error(err))
		return false, nil
	}
	if len(b) >= 5 && bytes.HasPrefix(b[1:5], []byte{0, 0, 0, 1}) {
		return qh.processPacketConn(pc)
	}
	return false, nil
}
