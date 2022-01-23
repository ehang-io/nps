package enet

import (
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/pool"
	"github.com/pkg/errors"
	"net"
)

var packetBp = pool.NewBufferPool(1500)

type S5PacketConn struct {
	net.PacketConn
	remoteAddr net.Addr
}

func NewS5PacketConn(pc net.PacketConn, remoteAddr net.Addr) *S5PacketConn {
	return &S5PacketConn{PacketConn: pc, remoteAddr: remoteAddr}
}

func (s *S5PacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	b := packetBp.Get()
	defer packetBp.Put(b)
	n, addr, err = s.PacketConn.ReadFrom(b)
	if err != nil {
		return
	}
	var targetAddr common.Addr
	targetAddr, err = common.SplitAddr(b[3:])
	if err != nil {
		return
	}
	n = copy(p, b[3+len(targetAddr):n])
	addr, err = net.ResolveUDPAddr("udp", targetAddr.String())
	return
}

func (s *S5PacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	n = len(p)
	b := packetBp.Get()
	defer packetBp.Put(b)
	var sAddr common.Addr
	sAddr, err = common.ParseAddr(addr.String())
	if err != nil {
		return
	}
	copy(b[3:], sAddr)
	if (3 + len(sAddr) + len(p)) > len(b) {
		err = errors.Errorf("data too long(%d)", len(p))
		return
	}
	copy(b[3+len(sAddr):], p)
	_, err = s.PacketConn.WriteTo(b[:3+len(sAddr)+len(p)], s.remoteAddr)
	return
}
