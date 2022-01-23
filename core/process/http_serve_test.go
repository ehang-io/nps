package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHttpServeProcess(t *testing.T) {
	sAddr, err := startHttp(t)
	assert.NoError(t, err)
	h := &HttpServeProcess{
		DefaultProcess: DefaultProcess{},
		Host:           "127.0.0.1",
		RouteUrl:       "",
		HeaderModify:   map[string]string{"modify": "nps"},
		HostModify:     "ehang.io",
		AddOrigin:      true,
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
			go h.ProcessConn(enet.NewReaderConn(c))
		}
	}()
	rep, err := doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/header/modify"))
	assert.NoError(t, err)
	assert.Equal(t, "nps", rep)

	rep, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/host"))
	assert.NoError(t, err)
	assert.Equal(t, "ehang.io", rep)

	rep, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/origin/xff"))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rep)

	rep, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/origin/xri"))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rep)

	h.BasicAuth = map[string]string{"aaa": "bbb"}
	h.Init(ac)
	rep, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/now"))
	assert.Error(t, err)
	_, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/now"), "aaa", "bbb")
	assert.NoError(t, err)

	h.BasicAuth = map[string]string{}
	h.CacheTime = 100
	h.CachePath = []string{"/now"}
	h.Init(ac)
	var time1, time2 string
	time1, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	time2, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	assert.NotEmpty(t, time1)
	assert.Equal(t, time1, time2)

}
