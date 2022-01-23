package handler

import (
	"ehang.io/nps/lib/enet"
	"net/http"
)

type HttpHandler struct {
	DefaultHandler
}

func NewHttpHandler() *HttpHandler {
	return &HttpHandler{}
}

func (h *HttpHandler) GetName() string {
	return "http"
}

func (h *HttpHandler) GetZhName() string {
	return "http协议"
}

func (h *HttpHandler) HandleConn(b []byte, c enet.Conn) (bool, error) {
	switch string(b[:3]) {
	case http.MethodGet[:3], http.MethodHead[:3], http.MethodPost[:3], http.MethodPut[:3], http.MethodPatch[:3], http.MethodDelete[:3], http.MethodConnect[:3], http.MethodOptions[:3], http.MethodTrace[:3]:
		return h.processConn(c)
	}
	return false, nil
}
