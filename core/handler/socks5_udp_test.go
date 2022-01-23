package handler

import (
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/enet"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestSocks5Handle(t *testing.T) {
	h := Socks5UdpHandler{}
	rule := &testRule{}
	h.AddRule(rule)

	finish := make(chan struct{}, 0)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		buf := make([]byte, 1024)
		n, addr, err := pc.ReadFrom(buf)
		assert.NoError(t, err)
		rPc := enet.NewReaderPacketConn(nil, buf[:n], addr)
		res, err := h.HandlePacketConn(rPc)

		assert.NoError(t, err)
		assert.Equal(t, true, res)
		assert.Equal(t, true, rule.run)
		finish <- struct{}{}
	}()

	data := []byte("test")
	go func() {
		cPc, err := net.ListenPacket("udp", "127.0.0.1:0")
		assert.NoError(t, err)
		pAddr, err := common.ParseAddr("8.8.8.8:53")
		assert.NoError(t, err)
		b := append([]byte{0, 0, 0}, pAddr...)
		b = append(b, data...)
		_, err = cPc.WriteTo(b, pc.LocalAddr())
		assert.NoError(t, err)
	}()
	<-finish
}
