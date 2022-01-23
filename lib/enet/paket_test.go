package enet

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestTcpPacketConn(t *testing.T) {
	bs := bytes.Repeat([]byte{1}, 100)
	targetAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:53")
	assert.NoError(t, err)

	finish := make(chan struct{}, 0)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		conn, err := ln.Accept()
		assert.NoError(t, err)
		b := make([]byte, 1024)
		n, addr, err := NewTcpPacketConn(conn).ReadFrom(b)
		assert.NoError(t, err)

		assert.Equal(t, targetAddr, addr)
		assert.Equal(t, n, 100)
		finish <- struct{}{}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)

	_, err = NewTcpPacketConn(conn).WriteTo(bs, targetAddr)
	assert.NoError(t, err)

	<-finish
}

func TestPacketConn(t *testing.T) {
	finish := make(chan struct{}, 0)

	sPacketConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)

	cPacketConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)

	bPacketConn := NewReaderPacketConn(sPacketConn, nil, sPacketConn.LocalAddr())

	sendAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:53")
	assert.NoError(t, err)

	go func() {
		b := make([]byte, 1024)
		n, addr, err := bPacketConn.ReadFrom(b)
		assert.NoError(t, err)
		assert.Equal(t, sendAddr, addr)
		assert.Equal(t, n, 4)

		_, err = bPacketConn.WriteTo(bytes.Repeat(b[:n], 10), cPacketConn.LocalAddr())
		assert.NoError(t, err)

		finish <- struct{}{}
	}()

	err = bPacketConn.SendPacket([]byte{0, 0, 0, 0}, sendAddr)
	assert.NoError(t, err)

	b := make([]byte, 1024)
	n, addr, err := cPacketConn.ReadFrom(b)
	assert.NoError(t, err)
	assert.Equal(t, n, 40)
	assert.Equal(t, addr, sPacketConn.LocalAddr())

	<-finish

}
