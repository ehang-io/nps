package handler

import (
	"ehang.io/nps/lib/enet"
)

const (
	recordTypeHandshake uint8 = 22
	typeClientHello     uint8 = 1
)

type HttpsHandler struct {
	DefaultHandler
}

func NewHttpsHandler() *HttpsHandler {
	return &HttpsHandler{}
}

func (h *HttpsHandler) GetName() string {
	return "https"
}

func (h *HttpsHandler) GetZhName() string {
	return "https协议"
}

func (h *HttpsHandler) HandleConn(b []byte, c enet.Conn) (bool, error) {
	if b[0] == recordTypeHandshake{
		return h.processConn(c)
	}
	return false, nil
}
