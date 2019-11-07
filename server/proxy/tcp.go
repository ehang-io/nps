package proxy

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server/connection"
)

type TunnelModeServer struct {
	BaseServer
	process  process
	listener net.Listener
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge NetBridge, task *file.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	return conn.NewTcpListenerAndProcess(s.task.ServerIp+":"+strconv.Itoa(s.task.Port), func(c net.Conn) {
		if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
			logs.Warn("client id %d, task id %d,error %s, when tcp connection", s.task.Client.Id, s.task.Id, err.Error())
			c.Close()
			return
		}
		logs.Trace("new tcp connection,local port %d,client %d,remote address %s", s.task.Port, s.task.Client.Id, c.RemoteAddr())
		s.process(conn.NewConn(c), s)
		s.task.Client.AddConn()
	}, &s.listener)
}

//close
func (s *TunnelModeServer) Close() error {
	return s.listener.Close()
}

//web管理方式
type WebServer struct {
	BaseServer
}

//开始
func (s *WebServer) Start() error {
	p, _ := beego.AppConfig.Int("web_port")
	if p == 0 {
		stop := make(chan struct{})
		<-stop
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.SetStaticPath("/static", filepath.Join(common.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(common.GetRunPath(), "web", "views"))
	if l, err := connection.GetWebManagerListener(); err == nil {
		beego.InitBeforeHTTPRun()
		http.Serve(l, beego.BeeApp.Handlers)
	} else {
		logs.Error(err)
	}
	return errors.New("Web management startup failure")
}

func (s *WebServer) Close() error {
	return nil
}

//new
func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

type process func(c *conn.Conn, s *TunnelModeServer) error

//tcp proxy
func ProcessTunnel(c *conn.Conn, s *TunnelModeServer) error {
	targetAddr, err := s.task.Target.GetRandomTarget()
	if err != nil {
		c.Close()
		logs.Warn("tcp port %d ,client id %d,task id %d connect error %s", s.task.Port, s.task.Client.Id, s.task.Id, err.Error())
		return err
	}
	return s.DealClient(c, s.task.Client, targetAddr, nil, common.CONN_TCP, nil, s.task.Flow, s.task.Target.LocalProxy)
}

//http proxy
func ProcessHttp(c *conn.Conn, s *TunnelModeServer) error {
	_, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		logs.Info(err)
		return err
	}
	if r.Method == "CONNECT" {
		c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		rb = nil
	}
	if err := s.auth(r, c, s.task.Client.Cnf.U, s.task.Client.Cnf.P); err != nil {
		return err
	}
	return s.DealClient(c, s.task.Client, addr, rb, common.CONN_TCP, nil, s.task.Flow, s.task.Target.LocalProxy)
}
