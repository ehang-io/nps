package handler

import "ehang.io/nps/lib/enet"

type Socks5UdpHandler struct {
	DefaultHandler
}

func (sh *Socks5UdpHandler) GetName() string {
	return "socks5_udp"
}

func (sh *Socks5UdpHandler) GetZhName() string {
	return "socks5 udp协议"
}

func (sh *Socks5UdpHandler) HandlePacketConn(pc enet.PacketConn) (bool, error) {
	b, _, err := pc.FirstPacket()
	if err != nil {
		return true, err
	}
	if b[0] == 0 {
		return sh.processPacketConn(pc)
	}
	return false, nil
}
