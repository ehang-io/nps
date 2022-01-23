package handler

import (
	"ehang.io/nps/lib/enet"
)

type RdpHandler struct {
	DefaultHandler
}

func (rh *RdpHandler) GetName() string {
	return "rdp"
}

func (rh *RdpHandler) GetZhName() string {
	return "rdp协议"
}

func (rh *RdpHandler) HandleConn(b []byte, c enet.Conn) (bool, error) {
	if b[0] == 3 && b[1] == 0 {
		return rh.processConn(c)
	}
	return false, nil
}
