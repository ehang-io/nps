package proxy

import (
	"bufio"
	"crypto/tls"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/cache"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server/connection"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type httpServer struct {
	BaseServer
	httpPort      int
	httpsPort     int
	httpServer    *http.Server
	httpsServer   *http.Server
	httpsListener net.Listener
	useCache      bool
	cache         *cache.Cache
	cacheLen      int
}

func NewHttp(bridge *bridge.Bridge, c *file.Tunnel, httpPort, httpsPort int, useCache bool, cacheLen int) *httpServer {
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
	}
	if useCache {
		httpServer.cache = cache.New(cacheLen)
	}
	return httpServer
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
	s.httpHandle(conn.NewConn(c), r)
}

func (s *httpServer) httpHandle(c *conn.Conn, r *http.Request) {
	var (
		isConn     = false
		host       *file.Host
		target     net.Conn
		lastHost   *file.Host
		err        error
		connClient io.ReadWriteCloser
		scheme     = r.URL.Scheme
		lk         *conn.Link
		targetAddr string
		readReq    bool
		reqCh      = make(chan *http.Request)
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
			lk = conn.NewLink(common.CONN_TCP, targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, host.Target.LocalProxy)
			if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
				logs.Notice("connect to target %s error %s", lk.Host, err)
				break
			}
			connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
			isConn = false
			go func() {
				defer connClient.Close()
				defer c.Close()
				for {
					if resp, err := http.ReadResponse(bufio.NewReader(connClient), r); err != nil {
						return
					} else {
						r := <-reqCh
						//if the cache is start and the response is in the extension,store the response to the cache list
						if s.useCache && strings.Contains(r.URL.Path, ".") {
							b, err := httputil.DumpResponse(resp, true)
							if err != nil {
								return
							}
							c.Write(b)
							host.Flow.Add(0, int64(len(b)))
							s.cache.Add(filepath.Join(host.Host, r.URL.Path), b)
						} else {
							lenConn := conn.NewLenConn(c)
							if err := resp.Write(lenConn); err != nil {
								logs.Error(err)
								return
							}
							host.Flow.Add(0, int64(lenConn.Len))
						}
					}
				}
			}()
		} else if readReq {
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
		//if the cache start and the request is in the cache list, return the cache
		if s.useCache {
			if v, ok := s.cache.Get(filepath.Join(host.Host, r.URL.Path)); ok {
				n, err := c.Write(v.([]byte))
				if err != nil {
					break
				}
				logs.Trace("%s request, method %s, host %s, url %s, remote address %s, return cache", r.URL.Scheme, r.Method, r.Host, r.URL.Path, c.RemoteAddr().String())
				host.Flow.Add(0, int64(n))
				//if return cache and does not create a new conn with client and Connection is not set or close, close the connection.
				if connClient == nil && (strings.ToLower(r.Header.Get("Connection")) == "close" || strings.ToLower(r.Header.Get("Connection")) == "") {
					c.Close()
					break
				}
				readReq = true
				goto start
			}
		}
		if connClient == nil {
			isConn = true
			goto start
		}
		readReq = true
		//change the host and header and set proxy setting
		common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		logs.Trace("%s request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.URL.Path, c.RemoteAddr().String(), lk.Host)
		//write
		lenConn := conn.NewLenConn(connClient)
		if err := r.Write(lenConn); err != nil {
			logs.Error(err)
			break
		}
		host.Flow.Add(int64(lenConn.Len), 0)
		reqCh <- r
	}
end:
	if !readReq {
		s.writeConnFail(c.Conn)
	}
	c.Close()
	if target != nil {
		target.Close()
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
