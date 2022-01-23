package handler

import (
	"ehang.io/nps/lib/enet"
)

type TransparentHandler struct {
	DefaultHandler
}

func (ts *TransparentHandler) GetName() string {
	return "transparent"
}

func (ts *TransparentHandler) GetZhName() string {
	return "linux透明代理协议"
}

func (ts *TransparentHandler) HandleConn(b []byte, c enet.Conn) (bool, error) {
	return ts.processConn(c)
}
