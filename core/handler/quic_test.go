package handler

import (
	"crypto/tls"
	"ehang.io/nps/lib/enet"
	"github.com/lucas-clemente/quic-go"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHandleQUICPacket(t *testing.T) {
	h := QUICHandler{}
	rule := &testRule{}
	h.AddRule(rule)
	finish := make(chan struct{}, 0)
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)

	go func() {
		b := make([]byte, 1500)
		n, addr, err := packetConn.ReadFrom(b)
		assert.NoError(t, err)
		pc := enet.NewReaderPacketConn(nil, b[:n], packetConn.LocalAddr())
		assert.NoError(t, pc.SendPacket(b[:n], addr))
		res, err := h.HandlePacketConn(pc)

		assert.NoError(t, err)
		assert.Equal(t, true, res)
		assert.Equal(t, true, rule.run)
		finish <- struct{}{}
	}()
	go quic.DialAddr(packetConn.LocalAddr().String(), &tls.Config{}, nil)
	<-finish
}
