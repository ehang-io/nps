package proxy

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

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

func (s *httpServer) handleTunneling(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	c, rw, err := hijacker.Hijack()
	if err != nil {
		//原来会把 err.Error() 直接写回客户端，内部错误细节（含监听地址等）会外泄，
		//这里只回固定文案，细节留在服务端日志
		logs.Warn("hijack http connection from %s error %s", r.RemoteAddr, err.Error())
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		// without the return the nil conn is dereferenced right below
		return
	}
	// rw.Reader may already hold bytes of the next request, hand it over
	// instead of throwing them away
	s.handleHttp(conn.NewConn(c), r, rw.Reader)
}

func (s *httpServer) handleHttp(c *conn.Conn, r *http.Request, br *bufio.Reader) {
	var (
		host       *file.Host
		target     net.Conn
		err        error
		connClient io.ReadWriteCloser
		scheme     = r.URL.Scheme
		lk         *conn.Link
		targetAddr string
		lenConn    *conn.LenConn
		isReset    bool
		wg         sync.WaitGroup
		// one reader per direction for the whole connection: rebuilding a
		// bufio.Reader on every loop throws away buffered bytes, which breaks
		// pipelined requests and responses that arrive with the next one
		reqReader  *bufio.Reader
		respReader *bufio.Reader
	)
	if br == nil {
		br = bufio.NewReader(c)
	}
	reqReader = br
	defer func() {
		if connClient != nil {
			connClient.Close()
		} else {
			s.writeConnFail(c.Conn)
		}
		c.Close()
	}()
reset:
	if isReset {
		host.Client.AddConn()
	}
	if host, err = file.GetDb().GetInfoByHost(r.Host, r); err != nil {
		logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
		return
	}
	if err := s.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Warn("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		return
	}
	if !isReset {
		defer host.Client.AddConn()
	}
	if err = s.auth(r, c, host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
		logs.Warn("auth error", err, r.RemoteAddr)
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		logs.Warn(err.Error())
		return
	}
	lk = conn.NewLink("http", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, host.Target.LocalProxy)
	if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
		logs.Notice("connect to target %s error %s", lk.Host, err)
		return
	}
	connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
	respReader = bufio.NewReader(connClient)

	//read from inc-client
	//Add must happen before the goroutine starts, otherwise the main goroutine
	//can reach Wait() first and close the connection under the writer
	wg.Add(1)
	go func() {
		isReset = false
		defer connClient.Close()
		defer func() {
			wg.Done()
			if !isReset {
				c.Close()
			}
		}()
		for {
			if resp, err := http.ReadResponse(respReader, r); err != nil || resp == nil || r == nil {
				// if there got broken pipe, http.ReadResponse will get a nil
				return
			} else {
				//if the cache is start and the response is in the extension,store the response to the cache list
				if s.useCache && r.URL != nil && strings.Contains(r.URL.Path, ".") {
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

	for {
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
				if strings.ToLower(r.Header.Get("Connection")) == "close" || strings.ToLower(r.Header.Get("Connection")) == "" {
					break
				}
				goto readReq
			}
		}

		//change the host and header and set proxy setting
		common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String(), s.addOrigin)
		logs.Trace("%s request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.URL.Path, c.RemoteAddr().String(), lk.Host)
		//write
		lenConn = conn.NewLenConn(connClient)
		if err := r.Write(lenConn); err != nil {
			logs.Error(err)
			break
		}
		host.Flow.Add(int64(lenConn.Len), 0)

	readReq:
		//read req from connection
		if r, err = http.ReadRequest(reqReader); err != nil {
			break
		}
		r.URL.Scheme = scheme
		//What happened ，Why one character less???
		r.Method = resetReqMethod(r.Method)
		if hostTmp, err := file.GetDb().GetInfoByHost(r.Host, r); err != nil {
			logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
			break
		} else if host != hostTmp {
			host = hostTmp
			isReset = true
			connClient.Close()
			goto reset
		}
	}
	wg.Wait()
}

func resetReqMethod(method string) string {
	if method == "ET" {
		return "GET"
	}
	if method == "OST" {
		return "POST"
	}
	return method
}

func (s *httpServer) NewServer(port int, scheme string) *http.Server {
	return &http.Server{
		Addr: ":" + strconv.Itoa(port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//域名转发同样可能被公网扫描器盯上：先按 allow_ip/deny_ip 过滤来源
			if !CheckHttpSource(r.RemoteAddr) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			r.URL.Scheme = scheme
			s.handleTunneling(w, r)
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
