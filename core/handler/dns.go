package handler

import (
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

type DnsHandler struct {
	DefaultHandler
}

func (dh *DnsHandler) GetName() string {
	return "dns"
}

func (dh *DnsHandler) GetZhName() string {
	return "dns协议"
}

func (dh *DnsHandler) HandlePacketConn(pc enet.PacketConn) (bool, error) {
	b, _, err := pc.FirstPacket()
	if err != nil {
		logger.Warn("firstPacket error", zap.Error(err))
		return false, nil
	}
	m := new(dns.Msg)
	err = m.Unpack(b)
	if err != nil {
		logger.Debug("parse dns request error", zap.Error(err))
		return false, nil
	}
	return dh.processPacketConn(pc)
}
