package proxy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/cache"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/connection"
	"github.com/astaxie/beego/logs"
)

type httpServer struct {
	BaseServer
	httpPort      int
	httpsPort     int
	httpServer    *http.Server
	httpsServer   *http.Server
	httpsListener net.Listener
	useCache      bool
	addOrigin     bool
	cache         *cache.Cache
	cacheLen      int
}

func NewHttp(bridge *bridge.Bridge, c *file.Tunnel, httpPort, httpsPort int, useCache bool, cacheLen int, addOrigin bool) *httpServer {
	httpServer := &httpServer{
		BaseServer: BaseServer{
			task:   c,
			bridge: bridge,
			Mutex:  sync.Mutex{},
		},
		httpPort:  httpPort,
		httpsPort: httpsPort,
		useCache:  useCache,
		cacheLen:  cacheLen,
		addOrigin: addOrigin,
	}
	if useCache {
		httpServer.cache = cache.New(cacheLen)
	}
	return httpServer
}

func (s *httpServer) Start() error {
	var err error
	if s.errorContent, err = common.ReadAllFromFile(filepath.Join(common.GetRunPath(), "web", "static", "page", "error.html")); err != nil {
		s.errorContent = []byte("nps 404")
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
			logs.Error(NewHttpsServer(s.httpsListener, s.bridge, s.useCache, s.cacheLen).Start())
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

func (s *httpServer) NewServer(port int, scheme string) *http.Server {
	rProxy := NewHttpReverseProxy(s)
	return &http.Server{
		Addr: ":" + strconv.Itoa(port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Scheme = scheme
			rProxy.ServeHTTP(w, r)
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}

type HttpReverseProxy struct {
	proxy *ReverseProxy

	responseHeaderTimeout time.Duration
}

func (rp *HttpReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var (
		host       *file.Host
		targetAddr string
		err        error
	)
	if host, err = file.GetDb().GetInfoByHost(req.Host, req); err != nil {
		rw.WriteHeader(http.StatusNotFound)
		rw.Write([]byte(req.Host + " not found"))
		return
	}
	if host.Client.Cnf.U != "" && host.Client.Cnf.P != "" && !common.CheckAuth(req, host.Client.Cnf.U, host.Client.Cnf.P) {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte("Unauthorized"))
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		rw.Write([]byte("502 Bad Gateway"))
		return
	}
	req = req.WithContext(context.WithValue(req.Context(), "host", host))
	req = req.WithContext(context.WithValue(req.Context(), "target", targetAddr))
	req = req.WithContext(context.WithValue(req.Context(), "req", req))

	rp.proxy.ServeHTTP(rw, req)
}

func NewHttpReverseProxy(s *httpServer) *HttpReverseProxy {
	rp := &HttpReverseProxy{
		responseHeaderTimeout: 30 * time.Second,
	}
	local, _ := net.ResolveTCPAddr("tcp", "127.0.0.1")
	proxy := NewReverseProxy(&httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Host = r.Host
			if host, err := file.GetDb().GetInfoByHost(r.Host, r); err != nil {
				logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
				return
			} else {
				common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, "", false)
			}
		},
		Transport: &http.Transport{
			ResponseHeaderTimeout: rp.responseHeaderTimeout,
			DisableKeepAlives:     true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var (
					host       *file.Host
					target     net.Conn
					err        error
					connClient io.ReadWriteCloser
					targetAddr string
					lk         *conn.Link
				)

				r := ctx.Value("req").(*http.Request)
				host = ctx.Value("host").(*file.Host)
				targetAddr = ctx.Value("target").(string)

				lk = conn.NewLink("http", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, host.Target.LocalProxy)
				if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
					logs.Notice("connect to target %s error %s", lk.Host, err)
					return nil, NewHTTPError(http.StatusBadGateway, "Cannot connect to the server")
				}
				connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
				return &flowConn{
					ReadWriteCloser: connClient,
					fakeAddr:        local,
					host:            host,
				}, nil
			},
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			logs.Warn("do http proxy request error: %v", err)
			rw.WriteHeader(http.StatusNotFound)
		},
	})
	proxy.WebSocketDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var (
			host       *file.Host
			target     net.Conn
			err        error
			connClient io.ReadWriteCloser
			targetAddr string
			lk         *conn.Link
		)
		r := ctx.Value("req").(*http.Request)
		host = ctx.Value("host").(*file.Host)
		targetAddr = ctx.Value("target").(string)

		lk = conn.NewLink("tcp", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, host.Target.LocalProxy)
		if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
			logs.Notice("connect to target %s error %s", lk.Host, err)
			return nil, NewHTTPError(http.StatusBadGateway, "Cannot connect to the target")
		}
		connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
		return &flowConn{
			ReadWriteCloser: connClient,
			fakeAddr:        local,
			host:            host,
		}, nil
	}
	rp.proxy = proxy
	return rp
}

type flowConn struct {
	io.ReadWriteCloser
	fakeAddr net.Addr
	host     *file.Host
	flowIn   int64
	flowOut  int64
	once     sync.Once
}

func (c *flowConn) Read(p []byte) (n int, err error) {
	n, err = c.ReadWriteCloser.Read(p)
	c.flowIn += int64(n)
	return n, err
}

func (c *flowConn) Write(p []byte) (n int, err error) {
	n, err = c.ReadWriteCloser.Write(p)
	c.flowOut += int64(n)
	return n, err
}

func (c *flowConn) Close() error {
	c.once.Do(func() { c.host.Flow.Add(c.flowIn, c.flowOut) })
	return c.ReadWriteCloser.Close()
}

func (c *flowConn) LocalAddr() net.Addr { return c.fakeAddr }

func (c *flowConn) RemoteAddr() net.Addr { return c.fakeAddr }

func (*flowConn) SetDeadline(t time.Time) error { return nil }

func (*flowConn) SetReadDeadline(t time.Time) error { return nil }

func (*flowConn) SetWriteDeadline(t time.Time) error { return nil }
