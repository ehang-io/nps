package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHttpsRedirectProcess(t *testing.T) {
	sAddr, err := startHttps(t)
	assert.NoError(t, err)
	h := &HttpsRedirectProcess{
		Host: "ehang.io",
	}
	ac := &action.LocalAction{
		DefaultAction: action.DefaultAction{},
		TargetAddr:    []string{sAddr},
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
	_, err = doRequest(fmt.Sprintf("https://%s%s", ln.Addr().String(), "/now"))
	assert.Error(t, err)

	h.Host = "*.github.com"
	_, err = doRequest(fmt.Sprintf("https://%s%s", ln.Addr().String(), "/now"))
	assert.NoError(t, err)
}
