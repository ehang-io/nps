package proxy

import (
	"errors"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server/connection"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

type TunnelModeServer struct {
	BaseServer
	process  process
	listener *net.TCPListener
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge *bridge.Bridge, task *file.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.task.Port, ""})
	if err != nil {
		return err
	}
	for {
		c, err := s.listener.AcceptTCP()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			logs.Info(err)
			continue
		}
		if err := s.checkFlow(); err != nil {
			logs.Warn("client id %d  task id %d  error  %s", s.task.Client.Id, s.task.Id, err.Error())
			c.Close()
		}
		if s.task.Client.GetConn() {
			logs.Trace("New tcp connection,client %d,remote address %s", s.task.Client.Id, c.RemoteAddr())
			go s.process(conn.NewConn(c), s)
		} else {
			logs.Info("Connections exceed the current client %d limit", s.task.Client.Id)
			c.Close()
		}
	}
	return nil
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
	//if !common.TestTcpPort(p) {
	//	//	logs.Error("Web management port %d is occupied", p)
	//	//	os.Exit(0)
	//	//}
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

//tcp隧道模式
func ProcessTunnel(c *conn.Conn, s *TunnelModeServer) error {
	return s.DealClient(c, s.task.Target, nil, common.CONN_TCP)
}

//http代理模式
func ProcessHttp(c *conn.Conn, s *TunnelModeServer) error {
	_, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		logs.Info(err)
		return err
	}
	if r.Method == "CONNECT" {
		c.Write([]byte("HTTP/1.1 200 Connection Established\r\n"))
		rb = nil
	}
	if err := s.auth(r, c, s.task.Client.Cnf.U, s.task.Client.Cnf.P); err != nil {
		return err
	}
	return s.DealClient(c, addr, rb, common.CONN_TCP)
}
