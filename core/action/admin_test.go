package action

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestAdminRunConn(t *testing.T) {
	ac := &AdminAction{
		DefaultAction: DefaultAction{},
	}
	finish := make(chan struct{}, 0)
	go func() {
		_, err := GetAdminListener().Accept()
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
