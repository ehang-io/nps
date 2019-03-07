package proxy

import (
	"bufio"
	"crypto/tls"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server/connection"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type httpServer struct {
	BaseServer
	httpPort      int //http端口
	httpsPort     int //https监听端口
	pemPath       string
	keyPath       string
	stop          chan bool
	httpslistener net.Listener
}

func NewHttp(bridge *bridge.Bridge, c *file.Tunnel) *httpServer {
	httpPort, _ := beego.AppConfig.Int("http_proxy_port")
	httpsPort, _ := beego.AppConfig.Int("https_proxy_port")
	pemPath := beego.AppConfig.String("pem_path")
	keyPath := beego.AppConfig.String("key_path")
	return &httpServer{
		BaseServer: BaseServer{
			task:   c,
			bridge: bridge,
			Mutex:  sync.Mutex{},
		},
		httpPort:  httpPort,
		httpsPort: httpsPort,
		pemPath:   pemPath,
		keyPath:   keyPath,
		stop:      make(chan bool),
	}
}

func (s *httpServer) Start() error {
	var err error
	var httpSrv, httpsSrv *http.Server
	if s.errorContent, err = common.ReadAllFromFile(filepath.Join(common.GetRunPath(), "web", "static", "page", "error.html")); err != nil {
		s.errorContent = []byte("easyProxy 404")
	}

	if s.httpPort > 0 {
		httpSrv = s.NewServer(s.httpPort, "http")
		go func() {
			l, err := connection.GetHttpListener()
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
			err = httpSrv.Serve(l)
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
		}()
	}
	if s.httpsPort > 0 {
		if !common.FileExists(s.pemPath) {
			os.Exit(0)
		}
		if !common.FileExists(s.keyPath) {
			logs.Error("ssl keyFile %s exist", s.keyPath)
			os.Exit(0)
		}
		httpsSrv = s.NewServer(s.httpsPort, "https")
		go func() {
			logs.Info("Start https listener, port is", s.httpsPort)
			l, err := connection.GetHttpsListener()
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
			err = httpsSrv.ServeTLS(l, s.pemPath, s.keyPath)
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
		}()
	}
	select {
	case <-s.stop:
		if httpSrv != nil {
			httpsSrv.Close()
		}
		if httpsSrv != nil {
			httpsSrv.Close()
		}
	}
	return nil
}

func (s *httpServer) Close() error {
	s.stop <- true
	return nil
}

func (s *httpServer) handleTunneling(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	c, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	s.process(conn.NewConn(c), r)
}

func (s *httpServer) process(c *conn.Conn, r *http.Request) {
	//多客户端域名代理
	var (
		isConn     = true
		host       *file.Host
		target     net.Conn
		lastHost   *file.Host
		err        error
		connClient io.ReadWriteCloser
		scheme     = r.URL.Scheme
	)
	if host, err = file.GetCsvDb().GetInfoByHost(r.Host, r); err != nil {
		logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
		goto end
	} else if !host.Client.GetConn() { //conn num limit
		logs.Notice("connections exceed the current client %d limit %d ,now connection num %d", host.Client.Id, host.Client.MaxConn, host.Client.NowConn)
		c.Close()
		return
	} else {
		logs.Trace("new %s connection,clientId %d,host %s,url %s,remote address %s", r.URL.Scheme, host.Client.Id, r.Host, r.URL, r.RemoteAddr)
		lastHost = host
	}
	for {
	start:
		if isConn {
			//流量限制
			if host.Client.Flow.FlowLimit > 0 && (host.Client.Flow.FlowLimit<<20) < (host.Client.Flow.ExportFlow+host.Client.Flow.InletFlow) {
				logs.Warn("Traffic exceeded client id %s", host.Client.Id)
				break
			}
			//权限控制
			if err = s.auth(r, c, host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
				logs.Warn("auth error", err, r.RemoteAddr)
				break
			}
			lk := conn.NewLink(common.CONN_TCP, host.GetRandomTarget(), host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr)
			if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, c.Conn.RemoteAddr().String(), nil); err != nil {
				logs.Notice("connect to target %s error %s", lk.Host, err)
				break
			}
			connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
			isConn = false
			go func() {
				w, _ := common.CopyBuffer(c, connClient)
				host.Flow.Add(0, w)
				c.Close()
				target.Close()
			}()
		} else {
			r, err = http.ReadRequest(bufio.NewReader(c))
			if err != nil {
				break
			}
			r.URL.Scheme = scheme
			//What happened ，Why one character less???
			if r.Method == "ET" {
				r.Method = "GET"
			}
			if r.Method == "OST" {
				r.Method = "POST"
			}
			logs.Trace("new %s connection,clientId %d,host %s,url %s,remote address %s", r.URL.Scheme, host.Client.Id, r.Host, r.URL, r.RemoteAddr)
			if hostTmp, err := file.GetCsvDb().GetInfoByHost(r.Host, r); err != nil {
				logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
				break
			} else if host != lastHost {
				host.Client.AddConn()
				if !hostTmp.Client.GetConn() {
					break
				}
				host = hostTmp
				lastHost = host
				isConn = true

				goto start
			}
		}
		//根据设定，修改header和host
		common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			break
		}
		host.Flow.Add(int64(len(b)), 0)
		logs.Trace("%s request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.RequestURI, r.RemoteAddr, host.Target)
		//write
		connClient.Write(b)
	}
end:
	if isConn {
		s.writeConnFail(c.Conn)
	}
	c.Close()
	if target != nil {
		target.Close()
	}
	if host != nil {
		host.Client.AddConn()
	}
}

func (s *httpServer) NewServer(port int, scheme string) *http.Server {
	return &http.Server{
		Addr: ":" + strconv.Itoa(port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Scheme = scheme
			s.handleTunneling(w, r)
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
