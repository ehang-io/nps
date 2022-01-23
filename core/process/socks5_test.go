package process

import (
	"crypto/tls"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/enet"
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestSocks5ProxyProcess(t *testing.T) {
	sAddr, err := startHttps(t)
	assert.NoError(t, err)
	h := Socks5Process{
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
			return url.Parse(fmt.Sprintf("socks5://%s", ln.Addr().String()))
		},
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestSocks5ProxyProcessAuth(t *testing.T) {
	sAddr, err := startHttps(t)
	h := Socks5Process{
		DefaultProcess: DefaultProcess{},
		Accounts:       map[string]string{"aaa": "bbb"},
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
			go func() {
				_, _ = h.ProcessConn(enet.NewReaderConn(c))
				_ = c.Close()
			}()
		}
	}()

	auth := proxy.Auth{
		User:     "aaa",
		Password: "bbb",
	}

	dialer, err := proxy.SOCKS5("tcp", ln.Addr().String(), nil, proxy.Direct)
	assert.NoError(t, err)

	tr := &http.Transport{Dial: dialer.Dial, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{
		Transport: tr,
	}

	resp, err := client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.Error(t, err)

	dialer, err = proxy.SOCKS5("tcp", ln.Addr().String(), &auth, proxy.Direct)
	assert.NoError(t, err)

	tr = &http.Transport{Dial: dialer.Dial, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client = &http.Client{
		Transport: tr,
	}

	resp, err = client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestSocks5ProxyProcessUdp(t *testing.T) {
	h := Socks5Process{
		DefaultProcess: DefaultProcess{},
	}
	ac := &action.LocalAction{}
	ac.Init()
	assert.NoError(t, h.Init(ac))
	h.ipStore.Set("127.0.0.1", true, time.Minute)

	serverPc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	localPc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	appPc, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	data := []byte("test")
	go func() {
		p := make([]byte, 1500)
		n, addr, err := appPc.ReadFrom(p)
		assert.NoError(t, err)
		assert.Equal(t, p[:n], data)
		_, err = appPc.WriteTo(data, addr)
		assert.NoError(t, err)
	}()
	go func() {
		p := make([]byte, 1500)
		n, addr, err := serverPc.ReadFrom(p)
		assert.NoError(t, err)
		pc := enet.NewReaderPacketConn(serverPc, p[:n], addr)
		err = pc.SendPacket(p[:n], addr)
		assert.NoError(t, err)
		b, err := h.ProcessPacketConn(pc)
		assert.Equal(t, b, true)
		assert.NoError(t, err)
	}()
	b := []byte{0, 0, 0}
	pAddr, err := common.ParseAddr(appPc.LocalAddr().String())
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
	assert.Equal(t, respAddr.String(), appPc.LocalAddr().String())
	assert.Equal(t, p[3+len(respAddr):n], data)
}
