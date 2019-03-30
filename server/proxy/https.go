package proxy

import (
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/pkg/errors"
	"net"
	"net/http"
	"net/url"
	"sync"
)

type HttpsServer struct {
	httpServer
	listener         net.Listener
	httpsListenerMap sync.Map
}

func NewHttpsServer(l net.Listener, bridge *bridge.Bridge) *HttpsServer {
	https := &HttpsServer{listener: l}
	https.bridge = bridge
	return https
}

func (https *HttpsServer) Start() error {
	if b, err := beego.AppConfig.Bool("https_just_proxy"); err == nil && b {
		conn.Accept(https.listener, func(c net.Conn) {
			https.handleHttps(c)
		})
	} else {
		conn.Accept(https.listener, func(c net.Conn) {
			serverName, rb := GetServerNameFromClientHello(c)
			var l *HttpsListener
			if v, ok := https.httpsListenerMap.Load(serverName); ok {
				l = v.(*HttpsListener)
			} else {
				r := buildHttpsRequest(serverName)
				if host, err := file.GetDb().GetInfoByHost(serverName, r); err != nil {
					c.Close()
					logs.Notice("the url %s can't be parsed!", serverName)
					return
				} else {
					if !common.FileExists(host.CertFilePath) || !common.FileExists(host.KeyFilePath) {
						c.Close()
						logs.Error("the key %s cert %s file is not exist", host.KeyFilePath, host.CertFilePath)
						return
					}
					l = NewHttpsListener(https.listener)
					https.NewHttps(l, host.CertFilePath, host.KeyFilePath)
					https.httpsListenerMap.Store(serverName, l)
				}
			}
			acceptConn := conn.NewConn(c)
			acceptConn.Rb = rb
			l.acceptConn <- acceptConn
		})
	}
	return nil
}

func (https *HttpsServer) Close() error {
	return https.listener.Close()
}

func (https *HttpsServer) NewHttps(l net.Listener, certFile string, keyFile string) {
	go func() {
		logs.Error(https.NewServer(0, "https").ServeTLS(l, certFile, keyFile))
	}()
}

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
	https.DealClient(conn.NewConn(c), host.Client, targetAddr, rb, common.CONN_TCP, nil, host.Flow)
}

type HttpsListener struct {
	acceptConn     chan *conn.Conn
	parentListener net.Listener
}

func NewHttpsListener(l net.Listener) *HttpsListener {
	return &HttpsListener{parentListener: l, acceptConn: make(chan *conn.Conn)}
}

func (httpsListener *HttpsListener) Accept() (net.Conn, error) {
	httpsConn := <-httpsListener.acceptConn
	if httpsConn == nil {
		return nil, errors.New("get connection error")
	}
	return httpsConn, nil
}

func (httpsListener *HttpsListener) Close() error {
	return nil
}

func (httpsListener *HttpsListener) Addr() net.Addr {
	return httpsListener.parentListener.Addr()
}

func GetServerNameFromClientHello(c net.Conn) (string, []byte) {
	buf := make([]byte, 4096)
	data := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		return "", nil
	}
	copy(data, buf[:n])
	clientHello := new(crypt.ClientHelloMsg)
	clientHello.Unmarshal(data[5:n])
	return clientHello.GetServerName(), buf[:n]
}

func buildHttpsRequest(hostName string) *http.Request {
	r := new(http.Request)
	r.RequestURI = "/"
	r.URL = new(url.URL)
	r.URL.Scheme = "https"
	r.Host = hostName
	return r
}
