package process

import (
	"bufio"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"encoding/base64"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"strings"
)

type HttpProxyProcess struct {
	DefaultProcess
	BasicAuth map[string]string `json:"basic_auth" placeholder:"username1 password1\nusername2 password2"  zh_name:"basic认证"`
}

func (hpp *HttpProxyProcess) GetName() string {
	return "http_proxy"
}

func (hpp *HttpProxyProcess) GetZhName() string {
	return "http代理"
}

func (hpp *HttpProxyProcess) ProcessConn(c enet.Conn) (bool, error) {
	r, err := http.ReadRequest(bufio.NewReader(c))
	if err != nil {
		return false, errors.Wrap(err, "read proxy request")
	}
	if len(hpp.BasicAuth) != 0 && !hpp.checkAuth(r) {
		return true, hpp.response(http.StatusProxyAuthRequired, map[string]string{"Proxy-Authenticate": "Basic realm=" + strconv.Quote("nps")}, c)
	}
	if r.Method == "CONNECT" {
		err = hpp.response(200, map[string]string{}, c)
		if err != nil {
			return true, errors.Wrap(err, "http proxy response")
		}
	} else if err = c.Reset(0); err != nil {
		logger.Warn("reset enet.Conn error", zap.Error(err))
		return true, err
	}
	address := r.Host
	if !strings.Contains(r.Host, ":") {
		if r.URL.Scheme == "https" {
			address = r.Host + ":443"
		} else {
			address = r.Host + ":80"
		}
	}
	return true, hpp.ac.RunConnWithAddr(c, address)
}
func (hpp *HttpProxyProcess) response(statusCode int, headers map[string]string, c enet.Conn) error {
	resp := &http.Response{
		Status:     http.StatusText(statusCode),
		StatusCode: statusCode,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}
	for k, v := range headers {
		resp.Header.Set(k, v)
	}
	return resp.Write(c)
}

func (hpp *HttpProxyProcess) checkAuth(r *http.Request) bool {
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 {
		s = strings.SplitN(r.Header.Get("Proxy-Authorization"), " ", 2)
		if len(s) != 2 {
			return false
		}
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return false
	}
	for u, p := range hpp.BasicAuth {
		if pair[0] == u && pair[1] == p {
			return true
		}
	}
	return false
}
