package enet

import (
	"ehang.io/nps/lib/common"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestNewS5PacketConn(t *testing.T) {
	serverPc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	localPc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	appAddr, err := net.ResolveUDPAddr("udp", "8.8.8.8:53")
	assert.NoError(t, err)
	data := []byte("test")
	go func() {
		p := make([]byte, 1500)
		n, addr, err := serverPc.ReadFrom(p)
		assert.NoError(t, err)
		pc := NewReaderPacketConn(serverPc, p[:n], addr)
		err = pc.SendPacket(p[:n], addr)
		assert.NoError(t, err)

		_, addr, err = pc.FirstPacket()
		assert.NoError(t, err)
		s5Pc := NewS5PacketConn(pc, addr)
		n, addr, err = s5Pc.ReadFrom(p)
		assert.NoError(t, err)
		assert.Equal(t, data, p[:n])
		assert.Equal(t, addr.String(), "8.8.8.8:53")
		_, err = s5Pc.WriteTo(data, appAddr)
		assert.NoError(t, err)
	}()
	b := []byte{0, 0, 0}
	pAddr, err := common.ParseAddr(appAddr.String())
	assert.NoError(t, err)
	b = append(b, pAddr...)
	b = append(b, data...)
	_, err = localPc.WriteTo(b, serverPc.LocalAddr())
	assert.NoError(t, err)
	p := make([]byte, 1500)
	n, _, err := localPc.ReadFrom(p)
	assert.NoError(t, err)
	respAddr, err := common.SplitAddr(p[3:])
	assert.NoError(t, err)
	assert.Equal(t, respAddr.String(), appAddr.String())
	assert.Equal(t, p[3+len(respAddr):n], data)
}
