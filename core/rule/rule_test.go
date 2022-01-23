package rule

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/limiter"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/server"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestRule(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	r := &Rule{
		Server:   &server.TcpServer{ServerAddr: "127.0.0.1:0"},
		Handler:  &handler.DefaultHandler{},
		Process:  &process.DefaultProcess{},
		Action:   &action.LocalAction{TargetAddr: []string{ln.Addr().String()}},
		Limiters: make([]limiter.Limiter, 0),
	}
	err = r.Init()
	assert.NoError(t, err)
	data := []byte("test")
	go func() {
		conn, err := ln.Accept()
		assert.NoError(t, err)
		b := make([]byte, 1024)
		n, err := conn.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, data, b[:n])
		_, err = conn.Write(b[:n])
		assert.NoError(t, err)
	}()
	conn, err := net.Dial(r.Server.GetName(), r.Server.GetServerAddr())
	assert.NoError(t, err)
	_, err = conn.Write(data)
	assert.NoError(t, err)
	b := make([]byte, 1024)
	n, err := conn.Read(b)
	assert.NoError(t, err)
	assert.Equal(t, b[:n], data)
}
