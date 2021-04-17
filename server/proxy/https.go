package proxy

import (
	"net"
	"net/http"
	"net/url"
	"sync"

	"ehang.io/nps/lib/cache"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/pkg/errors"
)

type HttpsServer struct {
	httpServer
	listener         net.Listener
	httpsListenerMap sync.Map
}

func NewHttpsServer(l net.Listener, bridge NetBridge, useCache bool, cacheLen int) *HttpsServer {
	https := &HttpsServer{listener: l}
	https.bridge = bridge
	https.useCache = useCache
	if useCache {
		https.cache = cache.New(cacheLen)
	}
	return https
}

//start https server
func (https *HttpsServer) Start() error {
	if b, err := beego.AppConfig.Bool("https_just_proxy"); err == nil && b {
		conn.Accept(https.listener, func(c net.Conn) {
			https.handleHttps(c)
		})
	} else {
		//start the default listener
		certFile := beego.AppConfig.String("https_default_cert_file")
		keyFile := beego.AppConfig.String("https_default_key_file")
		if common.FileExists(certFile) && common.FileExists(keyFile) {
			l := NewHttpsListener(https.listener)
			https.NewHttps(l, certFile, keyFile)
			https.httpsListenerMap.Store("default", l)
		}
		conn.Accept(https.listener, func(c net.Conn) {
			serverName, rb := GetServerNameFromClientHello(c)
			//if the clientHello does not contains sni ,use the default ssl certificate
			if serverName == "" {
				serverName = "default"
			}
			var l *HttpsListener
			if v, ok := https.httpsListenerMap.Load(serverName); ok {
				l = v.(*HttpsListener)
			} else {
				r := buildHttpsRequest(serverName)
				if host, err := file.GetDb().GetInfoByHost(serverName, r); err != nil {
					c.Close()
					logs.Notice("the url %s can't be parsed!,remote addr %s", serverName, c.RemoteAddr().String())
					return
				} else {
					if !common.FileExists(host.CertFilePath) || !common.FileExists(host.KeyFilePath) {
						//if the host cert file or key file is not set ,use the default file
						if v, ok := https.httpsListenerMap.Load("default"); ok {
							l = v.(*HttpsListener)
						} else {
							c.Close()
							logs.Error("the key %s cert %s file is not exist", host.KeyFilePath, host.CertFilePath)
							return
						}
					} else {
						l = NewHttpsListener(https.listener)
						https.NewHttps(l, host.CertFilePath, host.KeyFilePath)
						https.httpsListenerMap.Store(serverName, l)
					}
				}
			}
			acceptConn := conn.NewConn(c)
			acceptConn.Rb = rb
			l.acceptConn <- acceptConn
		})
	}
	return nil
}

// close
func (https *HttpsServer) Close() error {
	return https.listener.Close()
}

// new https server by cert and key file
func (https *HttpsServer) NewHttps(l net.Listener, certFile string, keyFile string) {
	go func() {
		logs.Error(https.NewServer(0, "https").ServeTLS(l, certFile, keyFile))
	}()
}

//handle the https which is just proxy to other client
func (https *HttpsServer) handleHttps(c net.Conn) {
	hostName, rb := GetServerNameFromClientHello(c)
	var targetAddr string
	r := buildHttpsRequest(hostName)
	var host *file.Host
	var err error
	if host, err = file.GetDb().GetInfoByHost(hostName, r); err != nil {
		c.Close()
		logs.Notice("the url %s can't be parsed!", hostName)
		return
	}
	if err := https.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Warn("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	defer host.Client.AddConn()
	if err = https.auth(r, conn.NewConn(c), host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
		logs.Warn("auth error", err, r.RemoteAddr)
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		logs.Warn(err.Error())
	}
	logs.Trace("new https connection,clientId %d,host %s,remote address %s", host.Client.Id, r.Host, c.RemoteAddr().String())
	https.DealClient(conn.NewConn(c), host.Client, targetAddr, rb, common.CONN_TCP, nil, host.Flow, host.Target.LocalProxy)
}

type HttpsListener struct {
	acceptConn     chan *conn.Conn
	parentListener net.Listener
}

// https listener
func NewHttpsListener(l net.Listener) *HttpsListener {
	return &HttpsListener{parentListener: l, acceptConn: make(chan *conn.Conn)}
}

// accept
func (httpsListener *HttpsListener) Accept() (net.Conn, error) {
	httpsConn := <-httpsListener.acceptConn
	if httpsConn == nil {
		return nil, errors.New("get connection error")
	}
	return httpsConn, nil
}

// close
func (httpsListener *HttpsListener) Close() error {
	return nil
}

// addr
func (httpsListener *HttpsListener) Addr() net.Addr {
	return httpsListener.parentListener.Addr()
}

// get server name from connection by read client hello bytes
func GetServerNameFromClientHello(c net.Conn) (string, []byte) {
	buf := make([]byte, 4096)
	data := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		return "", nil
	}
	if n < 42 {
		return "", nil
	}
	copy(data, buf[:n])
	clientHello := new(crypt.ClientHelloMsg)
	clientHello.Unmarshal(data[5:n])
	return clientHello.GetServerName(), buf[:n]
}

// build https request
func buildHttpsRequest(hostName string) *http.Request {
	r := new(http.Request)
	r.RequestURI = "/"
	r.URL = new(url.URL)
	r.URL.Scheme = "https"
	r.Host = hostName
	return r
}
