package handler

import (
	"ehang.io/nps/lib/enet"
	"github.com/icodeface/grdp"
	"github.com/icodeface/grdp/glog"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHandleRdpConn(t *testing.T) {
	h := RdpHandler{}
	rule := &testRule{}
	h.AddRule(rule)

	finish := make(chan struct{}, 0)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		conn, err := ln.Accept()
		assert.NoError(t, err)
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		assert.NoError(t, err)
		res, err := h.HandleConn(buf[:n], enet.NewReaderConn(conn))
		assert.NoError(t, err)
		assert.Equal(t, true, res)
		assert.Equal(t, true, rule.run)
		finish <- struct{}{}
	}()

	go func() {
		grdp.NewClient(ln.Addr().String(), glog.DEBUG).Login("Administrator", "123456")
	}()
	<-finish
}
