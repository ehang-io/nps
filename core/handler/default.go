package handler

import (
	"ehang.io/nps/lib/enet"
)

var (
	_ Handler = (*HttpHandler)(nil)
	_ Handler = (*HttpsHandler)(nil)
	_ Handler = (*RdpHandler)(nil)
	_ Handler = (*RedisHandler)(nil)
	_ Handler = (*Socks5Handler)(nil)
	_ Handler = (*TransparentHandler)(nil)
	_ Handler = (*DefaultHandler)(nil)
	_ Handler = (*DnsHandler)(nil)
	_ Handler = (*P2PHandler)(nil)
	_ Handler = (*QUICHandler)(nil)
	_ Handler = (*DefaultHandler)(nil)
	_ Handler = (*Socks5UdpHandler)(nil)
)

type RuleRun interface {
	RunConn(enet.Conn) (bool, error)
	RunPacketConn(enet.PacketConn) (bool, error)
}

type DefaultHandler struct {
	ruleList []RuleRun
}

func NewBaseTcpHandler() *DefaultHandler {
	return &DefaultHandler{ruleList: make([]RuleRun, 0)}
}

func (b *DefaultHandler) GetName() string {
	return "default"
}

func (b *DefaultHandler) GetZhName() string {
	return "默认"
}

func (b *DefaultHandler) HandleConn(_ []byte, c enet.Conn) (bool, error) {
	return b.processConn(c)
}

func (b *DefaultHandler) AddRule(r RuleRun) {
	b.ruleList = append(b.ruleList, r)
}

func (b *DefaultHandler) HandlePacketConn(_ enet.PacketConn) (bool, error) {
	return false, nil
}

func (b *DefaultHandler) processConn(c enet.Conn) (bool, error) {
	for _, r := range b.ruleList {
		if ok, err := r.RunConn(c); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func (b *DefaultHandler) processPacketConn(pc enet.PacketConn) (bool, error) {
	for _, r := range b.ruleList {
		if ok, err := r.RunPacketConn(pc); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}
