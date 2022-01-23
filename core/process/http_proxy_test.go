package process

import (
	"crypto/tls"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/url"
	"testing"
)

func TestHttpProxyProcess(t *testing.T) {
	sAddr, err := startHttps(t)
	assert.NoError(t, err)

	hsAddr, err := startHttp(t)
	assert.NoError(t, err)

	h := HttpProxyProcess{
		DefaultProcess: DefaultProcess{},
	}
	ac := &action.LocalAction{}
	ac.Init()
	assert.NoError(t, h.Init(ac))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go h.ProcessConn(enet.NewReaderConn(c))
		}
	}()

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse(fmt.Sprintf("http://%s", ln.Addr().String()))
		},
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	resp, err = client.Get(fmt.Sprintf("http://%s/now", hsAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHttpProxyProcessBasic(t *testing.T) {
	sAddr, err := startHttps(t)
	h := HttpProxyProcess{
		DefaultProcess: DefaultProcess{},
		BasicAuth:      map[string]string{"aaa": "bbb"},
	}
	ac := &action.LocalAction{}
	ac.Init()
	assert.NoError(t, h.Init(ac))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go h.ProcessConn(enet.NewReaderConn(c))
		}
	}()

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse(fmt.Sprintf("http://%s", ln.Addr().String()))
		},
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.Error(t, err)
	transport.Proxy = func(_ *http.Request) (*url.URL, error) {
		return url.Parse(fmt.Sprintf("http://%s:%s@%s", "aaa", "bbb", ln.Addr().String()))
	}

	resp, err = client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
