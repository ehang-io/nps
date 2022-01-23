package process

import (
	"bufio"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strings"
	"time"
)

// HttpServeProcess is proxy and modify http request
type HttpServeProcess struct {
	DefaultProcess
	tls          bool
	Host         string            `json:"host" required:"true" placeholder:"eg: www.nps.com or *.nps.com" zh_name:"域名"`
	RouteUrl     string            `json:"route_url" placeholder:"/api" zh_name:"匹配路径"`
	HeaderModify map[string]string `json:"header_modify" placeholder:"字段 修改值\nHost www.nps-change.com\nAccept */*"  zh_name:"请求头修改"`
	HostModify   string            `json:"host_modify" placeholder:"www.nps-changed.com" zh_name:"请求域名"`
	AddOrigin    bool              `json:"add_origin" zh_name:"添加来源"`
	CacheTime    int64             `json:"cache_time" placeholder:"600s" zh_name:"缓存时间"`
	CachePath    []string          `json:"cache_path" placeholder:".jd\n.css\n.png" zh_name:"缓存路径"`
	BasicAuth    map[string]string `json:"basic_auth" placeholder:"username1 password1\nusername2 password2" zh_name:"basic认证"`
	httpServe    *HttpServe
	ln           *enet.Listener
}

func (hp *HttpServeProcess) GetName() string {
	return "http_serve"
}

func (hp *HttpServeProcess) GetZhName() string {
	return "http服务"
}

// Init the action of process
func (hp *HttpServeProcess) Init(ac action.Action) error {
	hp.ac = ac
	hp.ln = enet.NewListener()
	hp.httpServe = NewHttpServe(hp.ln, ac)
	hp.httpServe.SetModify(hp.HeaderModify, hp.HostModify, hp.AddOrigin)
	if hp.CacheTime > 0 {
		hp.httpServe.SetCache(hp.CachePath, time.Duration(hp.CacheTime)*time.Second)
	}
	if len(hp.BasicAuth) != 0 {
		hp.httpServe.SetBasicAuth(hp.BasicAuth)
	}
	if !hp.tls {
		go hp.httpServe.Serve()
	}
	return nil
}

// ProcessConn is used to determine whether to hit the rule
// If true, send enet to httpServe
func (hp *HttpServeProcess) ProcessConn(c enet.Conn) (bool, error) {
	req, err := http.ReadRequest(bufio.NewReader(c))
	if err != nil {
		return false, errors.Wrap(err, "read request")
	}
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		return false, errors.Wrap(err, "split host")
	}
	if !(common.HostContains(hp.Host, host) && (hp.RouteUrl == "" || strings.HasPrefix(req.URL.Path, hp.RouteUrl))) {
		logger.Debug("do http proxy failed", zap.String("host", host), zap.String("url", hp.RouteUrl))
		return false, nil
	}
	logger.Debug("do http proxy", zap.String("host", host), zap.String("url", hp.RouteUrl))
	if err := c.Reset(0); err != nil {
		return true, errors.Wrap(err, "reset connection data")
	}
	return true, hp.ln.SendConn(c)
}
