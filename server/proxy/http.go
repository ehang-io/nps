package proxy

import (
	"bufio"
	"bytes"
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
	"net/url"
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
	httpServer    *http.Server
	httpsServer   *http.Server
	httpsListener net.Listener
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
	}
}

func (s *httpServer) processHttps(c net.Conn) {
	buf := make([]byte, 2048)
	n, err := c.Read(buf)
	if err != nil {
		return
	}
	var host *file.Host
	file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*file.Host)
		if v.Scheme != "https" && v.Scheme != "all" {
			return true
		}
		if bytes.Index(buf[:n], []byte(v.Host)) >= 0 && (host == nil || len(host.Host) < len(v.Host)) {
			host = v
			return false
		}
		return true
	})
	if host == nil {
		logs.Error("new https connection can't be parsed!", c.RemoteAddr().String())
		c.Close()
		return
	}
	var targetAddr string
	r := new(http.Request)
	r.RequestURI = "/"
	r.URL = new(url.URL)
	r.URL.Scheme = "https"
	r.Host = host.Host
	if err := s.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Warn("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	defer host.Client.AddConn()
	if err = s.auth(r, conn.NewConn(c), host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
		logs.Warn("auth error", err, r.RemoteAddr)
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		logs.Warn(err.Error())
	}
	logs.Trace("new https connection,clientId %d,host %s,remote address %s", host.Client.Id, r.Host, c.RemoteAddr().String())
	s.DealClient(conn.NewConn(c), host.Client, targetAddr, buf[:n], common.CONN_TCP, nil, host.Flow)
}

func (s *httpServer) Start() error {
	var err error
	if s.errorContent, err = common.ReadAllFromFile(filepath.Join(common.GetRunPath(), "web", "static", "page", "error.html")); err != nil {
		s.errorContent = []byte("easyProxy 404")
	}
	if s.httpPort > 0 {
		s.httpServer = s.NewServer(s.httpPort, "http")
		go func() {
			l, err := connection.GetHttpListener()
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
			err = s.httpServer.Serve(l)
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
		}()
	}
	if s.httpsPort > 0 {
		s.httpsServer = s.NewServer(s.httpsPort, "https")
		go func() {
			s.httpsListener, err = connection.GetHttpsListener()
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
			if b, err := beego.AppConfig.Bool("https_just_proxy"); err == nil && b {
				for {
					c, err := s.httpsListener.Accept()
					if err != nil {
						logs.Error(err)
						break
					}
					go s.processHttps(c)
				}
			} else {
				if !common.FileExists(s.pemPath) {
					logs.Error("ssl certFile %s exist", s.keyPath)
					os.Exit(0)
				}
				if !common.FileExists(s.keyPath) {
					logs.Error("ssl keyFile %s exist", s.keyPath)
					os.Exit(0)
				}
				err = s.httpsServer.ServeTLS(s.httpsListener, s.pemPath, s.keyPath)
				if err != nil {
					logs.Error(err)
					os.Exit(0)
				}
			}
		}()
	}
	return nil
}

func (s *httpServer) Close() error {
	if s.httpsListener != nil {
		s.httpsListener.Close()
	}
	if s.httpsServer != nil {
		s.httpsServer.Close()
	}
	if s.httpServer != nil {
		s.httpServer.Close()
	}
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
	var (
		isConn     = true
		host       *file.Host
		target     net.Conn
		lastHost   *file.Host
		err        error
		connClient io.ReadWriteCloser
		scheme     = r.URL.Scheme
		lk         *conn.Link
		targetAddr string
		wg         sync.WaitGroup
	)
	if host, err = file.GetDb().GetInfoByHost(r.Host, r); err != nil {
		logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
		goto end
	}
	if err := s.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Warn("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	defer host.Client.AddConn()
	logs.Trace("new %s connection,clientId %d,host %s,url %s,remote address %s", r.URL.Scheme, host.Client.Id, r.Host, r.URL, r.RemoteAddr)
	lastHost = host
	for {
	start:
		if isConn {
			if err = s.auth(r, c, host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
				logs.Warn("auth error", err, r.RemoteAddr)
				break
			}
			if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
				logs.Warn(err.Error())
				break
			}
			lk = conn.NewLink(common.CONN_TCP, targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr)
			if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, c.Conn.RemoteAddr().String(), nil); err != nil {
				logs.Notice("connect to target %s error %s", lk.Host, err)
				break
			}
			connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
			isConn = false
			go func() {
				wg.Add(1)
				w, _ := common.CopyBuffer(c, connClient)
				host.Flow.Add(0, w)
				c.Close()
				target.Close()
				wg.Done()
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
			if hostTmp, err := file.GetDb().GetInfoByHost(r.Host, r); err != nil {
				logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
				break
			} else if host != lastHost {
				host = hostTmp
				lastHost = host
				isConn = true
				goto start
			}
		}
		//根据设定，修改header和host
		common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		b, err := httputil.DumpRequest(r, false)
		if err != nil {
			break
		}
		logs.Trace("%s request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.RequestURI, r.RemoteAddr, lk.Host)
		//write
		connClient.Write(b)
		if bodyLen, err := common.CopyBuffer(connClient, r.Body); err != nil {
			break
		} else {
			host.Flow.Add(int64(len(b))+bodyLen, 0)
		}
	}
end:
	if isConn {
		s.writeConnFail(c.Conn)
	}
	c.Close()
	if target != nil {
		target.Close()
	}
	wg.Wait()
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
