package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/pb"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func TestPbPingProcess(t *testing.T) {

	h := &PbPingProcessor{}
	ac := &action.LocalAction{
		DefaultAction: action.DefaultAction{},
		TargetAddr:    []string{},
	}
	ac.Init()
	err = h.Init(ac)
	assert.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go func() {
				_, _ = h.ProcessConn(enet.NewReaderConn(c))
				_ = c.Close()
			}()
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)
	_, err = pb.WriteMessage(conn, &pb.Ping{Now: time.Now().String()})
	assert.NoError(t, err)
	m := &pb.Ping{}
	_, err = pb.ReadMessage(conn, m)
	assert.NoError(t, err)
	assert.NotEmpty(t, m.Now)
}
