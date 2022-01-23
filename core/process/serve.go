package process

import (
	"context"
	"crypto/tls"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/logger"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type HttpServe struct {
	engine       *gin.Engine
	ln           net.Listener
	ac           action.Action
	httpServe    *http.Server
	cacheStore   *persistence.InMemoryStore
	cacheTime    time.Duration
	cachePath    []string
	headerModify map[string]string
	hostModify   string
	addOrigin    bool
	reverseProxy *httputil.ReverseProxy
}

func NewHttpServe(ln net.Listener, ac action.Action) *HttpServe {
	gin.SetMode(gin.ReleaseMode)
	hs := &HttpServe{
		ln:     ln,
		ac:     ac,
		engine: gin.New(),
	}
	hs.httpServe = &http.Server{
		Handler:      hs.engine,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	hs.reverseProxy = &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			_ = hs.transport(request)
			hs.doModify(request)
		},
		Transport: &http.Transport{
			MaxIdleConns:    10000,
			IdleConnTimeout: 30 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ac.GetServerConn()
			},
		},
	}
	serverHttp := func(w http.ResponseWriter, r *http.Request) {
		hs.reverseProxy.ServeHTTP(w, r)
	}
	hs.engine.NoRoute(func(c *gin.Context) {
		cached := false
		for _, p := range hs.cachePath {
			if strings.Contains(c.Request.RequestURI, p) {
				cached = true
				cache.CachePage(hs.cacheStore, hs.cacheTime, func(c *gin.Context) {
					serverHttp(c.Writer, c.Request)
				})(c)
			}
		}
		if !cached {
			serverHttp(c.Writer, c.Request)
		}
	})
	hs.engine.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Debug("http request",
			zap.String("client_ip", param.ClientIP),
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.String("proto", param.Request.Proto),
			zap.Duration("latency", param.Latency),
			zap.String("user_agent", param.Request.UserAgent()),
			zap.String("error_message", param.ErrorMessage),
			zap.Int("response_code", param.StatusCode),
		)
		return ""
	}))
	return hs
}

func (hs *HttpServe) transport(req *http.Request) error {
	ruri := req.URL.RequestURI()
	req.URL.Scheme = "http"
	if req.URL.Scheme != "" && req.URL.Opaque == "" {
		ruri = req.URL.Scheme + "://" + req.Host + ruri
	} else if req.Method == "CONNECT" && req.URL.Path == "" {
		// CONNECT requests normally give just the host and port, not a full URL.
		ruri = req.Host
		if req.URL.Opaque != "" {
			ruri = req.URL.Opaque
		}
	}
	req.RequestURI = ""
	var err error
	req.URL, err = url.Parse(ruri)
	return err
}

func (hs *HttpServe) SetBasicAuth(accounts map[string]string) {
	hs.engine.Use(gin.BasicAuth(accounts), gin.Recovery())
}

func (hs *HttpServe) SetCache(cachePath []string, cacheTime time.Duration) {
	hs.cachePath = cachePath
	hs.cacheTime = cacheTime
	hs.cacheStore = persistence.NewInMemoryStore(cacheTime * time.Second)
}

func (hs *HttpServe) SetModify(headerModify map[string]string, hostModify string, addOrigin bool) {
	hs.headerModify = headerModify
	hs.hostModify = hostModify
	hs.addOrigin = addOrigin
	return
}

func (hs *HttpServe) Serve() error {
	return hs.httpServe.Serve(hs.ln)
}

func (hs *HttpServe) ServeTLS(certFile string, keyFile string) error {
	return hs.httpServe.ServeTLS(hs.ln, certFile, keyFile)
}

// doModify is used to modify http request
func (hs *HttpServe) doModify(req *http.Request) {
	if hs.hostModify != "" {
		req.Host = hs.hostModify
	}
	for k, v := range hs.headerModify {
		req.Header.Set(k, v)
	}
	addr := strings.Split(req.RemoteAddr, ":")[0]
	if hs.addOrigin {
		// XFF is setting in reverseProxy
		req.Header.Set("X-Real-IP", addr)
	}
}

func writeContentType(w http.ResponseWriter, value []string) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = value
	}
}

type render struct {
	resp *http.Response
}

func (r *render) Render(writer http.ResponseWriter) error {
	_, err := io.Copy(writer, r.resp.Body)
	return err
}

func (r *render) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, []string{r.resp.Header.Get("Content-Type")})
}
