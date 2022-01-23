package action

import (
	"ehang.io/nps/lib/enet"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestBridgeRunConn(t *testing.T) {
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	ac := &BridgeAction{
		DefaultAction:   DefaultAction{},
		WritePacketConn: packetConn,
	}
	finish := make(chan struct{}, 0)
	go func() {
		_, err := GetBridgeListener().Accept()
		assert.NoError(t, err)
		finish <- struct{}{}
	}()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		conn, err := ln.Accept()
		assert.NoError(t, err)
		assert.NoError(t, ac.RunConn(conn))
	}()
	_, err = net.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)
	<-finish
}

func TestBridgeRunPacket(t *testing.T) {
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	ac := &BridgeAction{
		DefaultAction:   DefaultAction{},
		WritePacketConn: packetConn,
	}
	assert.NoError(t, ac.Init())
	go func() {
		p := make([]byte, 1024)
		pc := GetBridgePacketConn()
		n, addr, err := pc.ReadFrom(p)
		assert.NoError(t, err)
		_, err = pc.WriteTo(p[:n], addr)
		assert.NoError(t, err)
	}()
	go func() {
		p := make([]byte, 1024)
		n, addr, err := packetConn.ReadFrom(p)
		assert.NoError(t, err)
		bPacketConn := enet.NewReaderPacketConn(packetConn, p[:n], addr)
		go func() {
			err = ac.RunPacketConn(bPacketConn)
			assert.NoError(t, err)
		}()
		err = bPacketConn.SendPacket(p[:n], addr)
		assert.NoError(t, err)
	}()
	cPacketConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	b := []byte("12345")
	_, err = cPacketConn.WriteTo(b, packetConn.LocalAddr())
	assert.NoError(t, err)
	p := make([]byte, 1024)
	n, addr, err := cPacketConn.ReadFrom(p)
	assert.NoError(t, err)
	assert.Equal(t, addr.String(), packetConn.LocalAddr().String())
	assert.Equal(t, p[:n], b)
}
