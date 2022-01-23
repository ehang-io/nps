package service

import "net"

type HttpService struct {
	ln net.Listener
}

func NewHttpService(ln net.Listener) *HttpService {
	return &HttpService{ln: ln}
}
