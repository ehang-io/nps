package handler

import "ehang.io/nps/lib/enet"

type Socks5Handler struct {
	DefaultHandler
}

func (sh *Socks5Handler) GetName() string {
	return "socks5"
}

func (sh *Socks5Handler) GetZhName() string {
	return "socks5协议"
}

func (sh *Socks5Handler) HandleConn(b []byte, c enet.Conn) (bool, error) {
	if b[0] == 5 {
		return sh.processConn(c)
	}
	return false, nil
}
