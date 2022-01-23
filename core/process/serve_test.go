package process

import (
	"context"
	"crypto/tls"
	"ehang.io/nps/core/action"
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

var startHttpOnce sync.Once
var startHttpsOnce sync.Once
var handleOnce sync.Once
var ln net.Listener
var lns net.Listener
var err error

func registerHandle() {
	handleOnce.Do(func() {
		http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
			msg := make([]byte, 512)
			n, err := ws.Read(msg)
			if err != nil {
				return
			}
			ws.Write(msg[:n])
		}))
		http.HandleFunc("/now", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(time.Now().String()))
		})
		http.HandleFunc("/host", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(r.Host))
		})
		http.HandleFunc("/header/modify", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(r.Header.Get("modify")))
		})
		http.HandleFunc("/origin/xff", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(r.Header.Get("X-Forwarded-For")))
		})
		http.HandleFunc("/origin/xri", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(r.Header.Get("X-Real-IP")))
		})
	})
}

func startHttp(t *testing.T) (string, error) {
	startHttpOnce.Do(func() {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return
		}
		registerHandle()
		go http.Serve(ln, nil)

	})

	return ln.Addr().String(), err
}

func startHttps(t *testing.T) (string, error) {
	startHttpsOnce.Do(func() {
		lns, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return
		}
		registerHandle()
		certFilePath, keyFilePath := createCertFile(t)
		go http.ServeTLS(lns, nil, certFilePath, keyFilePath)
	})

	return lns.Addr().String(), err
}

func doRequest(params ...string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest("GET", params[0], nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Connection", "close")
	if len(params) >= 3 && params[1] != "" {
		req.SetBasicAuth(params[1], params[2])
	}
	if req.URL.Scheme == "https" {
		client.Transport = &http.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return tls.Dial(network, addr, &tls.Config{
					InsecureSkipVerify: true,
					ServerName:         "www.github.com",
				})
			},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "0", errors.Errorf("respond error, code %d", resp.StatusCode)
	}
	return string(b), nil
}

func createHttpServe(serverAddr string) (*HttpServe, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ac := &action.LocalAction{
		DefaultAction: action.DefaultAction{},
		TargetAddr:    []string{serverAddr},
	}
	ac.Init()
	return NewHttpServe(ln, ac), nil
}

func TestHttpServeWebsocket(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)

	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	go hs.Serve()

	ws, err := websocket.Dial(fmt.Sprintf("ws://%s/ws", hs.ln.Addr().String()), "", fmt.Sprintf("http://%s/ws", hs.ln.Addr().String()))
	assert.NoError(t, err)

	defer ws.Close() //关闭连接

	sendMsg := []byte("nps")
	_, err = ws.Write(sendMsg)
	assert.NoError(t, err)

	msg := make([]byte, 512)
	m, err := ws.Read(msg)
	assert.NoError(t, err)

	assert.Equal(t, sendMsg, msg[:m])
}

func TestHttpsServeWebsocket(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)

	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	cert, key := createCertFile(t)
	go hs.ServeTLS(cert, key)

	config, err := websocket.NewConfig(fmt.Sprintf("wss://%s/ws", hs.ln.Addr().String()), fmt.Sprintf("https://%s/ws", hs.ln.Addr().String()))
	assert.NoError(t, err)
	config.TlsConfig = &tls.Config{InsecureSkipVerify: true}

	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)

	defer ws.Close() //关闭连接

	sendMsg := []byte("nps")
	_, err = ws.Write(sendMsg)
	assert.NoError(t, err)

	msg := make([]byte, 512)
	m, err := ws.Read(msg)
	assert.NoError(t, err)

	assert.Equal(t, sendMsg, msg[:m])
}

func TestHttpServeModify(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)

	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	go hs.Serve()

	hs.SetModify(map[string]string{"modify": "test"}, "ehang.io", true)

	rep, err := doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/header/modify"))
	assert.NoError(t, err)
	assert.Equal(t, "test", rep)

	rep, err = doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/host"))
	assert.NoError(t, err)
	assert.Equal(t, "ehang.io", rep)

	rep, err = doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/origin/xff"))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rep)

	rep, err = doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/origin/xri"))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rep)
}

func TestHttpsServeModify(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)

	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	cert, key := createCertFile(t)
	go hs.ServeTLS(cert, key)

	hs.SetModify(map[string]string{"modify": "test"}, "ehang.io", true)

	rep, err := doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/header/modify"))
	assert.NoError(t, err)
	assert.Equal(t, "test", rep)

	rep, err = doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/host"))
	assert.NoError(t, err)
	assert.Equal(t, "ehang.io", rep)

	rep, err = doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/origin/xff"))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rep)

	rep, err = doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/origin/xri"))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", rep)
}

func TestHttpServeCache(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)
	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	go hs.Serve()
	hs.SetCache([]string{"now"}, time.Second*10)

	var time1, time2 string
	time1, err = doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	time2, err = doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	assert.NotEmpty(t, time1)
	assert.Equal(t, time1, time2)
}

func TestHttpsServeCache(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)
	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	cert, key := createCertFile(t)
	go hs.ServeTLS(cert, key)
	hs.SetCache([]string{"now"}, time.Second*10)

	var time1, time2 string
	time1, err = doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	time2, err = doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	assert.NotEmpty(t, time1)
	assert.Equal(t, time1, time2)
}

func TestHttpServeBasicAuth(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)
	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	go hs.Serve()
	hs.SetBasicAuth(map[string]string{"aaa": "bbb"})
	_, err = doRequest(fmt.Sprintf("http://%s%s", hs.ln.Addr().String(), "/now"), "aaa", "bbb")
	assert.NoError(t, err)
}

func TestHttpsServeBasicAuth(t *testing.T) {
	serverAddr, err := startHttp(t)
	assert.NoError(t, err)
	hs, err := createHttpServe(serverAddr)
	assert.NoError(t, err)
	cert, key := createCertFile(t)
	go hs.ServeTLS(cert, key)

	hs.SetBasicAuth(map[string]string{"aaa": "bbb"})
	_, err = doRequest(fmt.Sprintf("https://%s%s", hs.ln.Addr().String(), "/now"), "aaa", "bbb")
	assert.NoError(t, err)
}
