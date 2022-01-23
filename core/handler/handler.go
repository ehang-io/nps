package handler

import "ehang.io/nps/lib/enet"

type Handler interface {
	GetName() string
	GetZhName() string
	AddRule(RuleRun)
	HandleConn([]byte, enet.Conn) (bool, error)
	HandlePacketConn(enet.PacketConn) (bool, error)
}
