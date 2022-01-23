package handler

import (
	"crypto/tls"
	"ehang.io/nps/lib/enet"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/url"
	"testing"
)

func TestHandleSocks5Conn(t *testing.T) {
	h := Socks5Handler{}
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
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy: func(_ *http.Request) (*url.URL, error) {
				return url.Parse(fmt.Sprintf("socks5://%s", ln.Addr().String()))
			},
		}

		client := &http.Client{Transport: transport}
		_, _ = client.Get("https://google.com/")
	}()
	<-finish
}
