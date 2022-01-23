package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
)

var (
	_ Process = (*DefaultProcess)(nil)
	_ Process = (*HttpServeProcess)(nil)
	_ Process = (*HttpsServeProcess)(nil)
	_ Process = (*HttpProxyProcess)(nil)
	_ Process = (*HttpsProxyProcess)(nil)
	_ Process = (*HttpsRedirectProcess)(nil)
	_ Process = (*Socks5Process)(nil)
	_ Process = (*TransparentProcess)(nil)
)

type Process interface {
	Init(action action.Action) error
	GetName() string
	GetZhName() string
	ProcessConn(enet.Conn) (bool, error)
	ProcessPacketConn(enet.PacketConn) (bool, error)
}

type DefaultProcess struct {
	ac action.Action
}

func (bp *DefaultProcess) ProcessConn(c enet.Conn) (bool, error) {
	return true, bp.ac.RunConn(c)
}

func (bp *DefaultProcess) GetName() string {
	return "default"
}

func (bp *DefaultProcess) GetZhName() string {
	return "默认"
}

// Init the action of process
func (bp *DefaultProcess) Init(ac action.Action) error {
	bp.ac = ac
	return nil
}

func (bp *DefaultProcess) ProcessPacketConn(pc enet.PacketConn) (bool, error) {
	return true, bp.ac.RunPacketConn(pc)
}
