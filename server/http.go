package server

import (
	"bufio"
	"crypto/tls"
	"github.com/astaxie/beego"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"strconv"
	"sync"
)

type httpServer struct {
	server
	httpPort  int //http端口
	httpsPort int //https监听端口
	pemPath   string
	keyPath   string
	stop      chan bool
}

func NewHttp(bridge *bridge.Bridge, c *lib.Tunnel) *httpServer {
	httpPort, _ := beego.AppConfig.Int("httpProxyPort")
	httpsPort, _ := beego.AppConfig.Int("httpsProxyPort")
	pemPath := beego.AppConfig.String("pemPath")
	keyPath := beego.AppConfig.String("keyPath")
	return &httpServer{
		server: server{
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
	var http, https *http.Server
	if s.errorContent, err = lib.ReadAllFromFile(filepath.Join(lib.GetRunPath(), "web", "static", "page", "error.html")); err != nil {
		s.errorContent = []byte("easyProxy 404")
	}

	if s.httpPort > 0 {
		http = s.NewServer(s.httpPort)
		go func() {
			lib.Println("启动http监听,端口为", s.httpPort)
			err := http.ListenAndServe()
			if err != nil {
				lib.Fatalln(err)
			}
		}()
	}
	if s.httpsPort > 0 {
		if !lib.FileExists(s.pemPath) {
			lib.Fatalf("ssl certFile文件%s不存在", s.pemPath)
		}
		if !lib.FileExists(s.keyPath) {
			lib.Fatalf("ssl keyFile文件%s不存在", s.keyPath)
		}
		https = s.NewServer(s.httpsPort)
		go func() {
			lib.Println("启动https监听,端口为", s.httpsPort)
			err := https.ListenAndServeTLS(s.pemPath, s.keyPath)
			if err != nil {
				lib.Fatalln(err)
			}
		}()
	}
	select {
	case <-s.stop:
		if http != nil {
			http.Close()
		}
		if https != nil {
			https.Close()
		}
	}
	return nil
}

func (s *httpServer) Close() {
	s.stop <- true
}

func (s *httpServer) handleTunneling(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	s.process(lib.NewConn(conn), r)
}

func (s *httpServer) process(c *lib.Conn, r *http.Request) {
	//多客户端域名代理
	var (
		isConn = true
		link   *lib.Link
		host   *lib.Host
		tunnel *lib.Conn
		err    error
	)
	for {
		//首次获取conn
		if isConn {
			if host, err = GetInfoByHost(r.Host); err != nil {
				lib.Printf("the host %s is not found !", r.Host)
				break
			}
			//流量限制
			if host.Client.Flow.FlowLimit > 0 && (host.Client.Flow.FlowLimit<<20) < (host.Client.Flow.ExportFlow+host.Client.Flow.InletFlow) {
				break
			}
			host.Client.Cnf.CompressDecode, host.Client.Cnf.CompressEncode = lib.GetCompressType(host.Client.Cnf.Compress)
			//权限控制
			if err = s.auth(r, c, host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
				break
			}
			link = lib.NewLink(host.Client.GetId(), lib.CONN_TCP, host.GetRandomTarget(), host.Client.Cnf.CompressEncode, host.Client.Cnf.CompressDecode, host.Client.Cnf.Crypt, c, host.Flow, nil, host.Client.Rate, nil)
			if tunnel, err = s.bridge.SendLinkInfo(host.Client.Id, link); err != nil {
				break
			}
			isConn = false
		} else {
			r, err = http.ReadRequest(bufio.NewReader(c))
			if err != nil {
				break
			}
		}
		//根据设定，修改header和host
		lib.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			break
		}
		host.Flow.Add(len(b), 0)
		if _, err := tunnel.SendMsg(b, link); err != nil {
			c.Close()
			break
		}
	}

	if isConn {
		s.writeConnFail(c.Conn)
	} else {
		tunnel.SendMsg([]byte(lib.IO_EOF), link)
	}

	c.Close()

}

func (s *httpServer) NewServer(port int) *http.Server {
	return &http.Server{
		Addr: ":" + strconv.Itoa(port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.handleTunneling(w, r)
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
