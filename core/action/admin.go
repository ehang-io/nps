package action

import (
	"ehang.io/nps/lib/enet"
	"net"
)

var adminListener = enet.NewListener()

func GetAdminListener() net.Listener {
	return adminListener
}

type AdminAction struct {
	DefaultAction
}

func (la *AdminAction) GetName() string {
	return "admin"
}

func (la *AdminAction) GetZhName() string {
	return "转发到控制台"
}

func (la *AdminAction) RunConn(clientConn net.Conn) error {
	return adminListener.SendConn(clientConn)
}

func (la *AdminAction) RunConnWithAddr(clientConn net.Conn, addr string) error {
	return adminListener.SendConn(clientConn)
}
