package proxy

import (
	"errors"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"net"
	"path/filepath"
	"strings"
)

type TunnelModeServer struct {
	server
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
			lg.Println(err)
			continue
		}
		go s.process(conn.NewConn(c), s)
	}
	return nil
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *conn.Conn, cnf *file.Config, addr string, method string, rb []byte) error {
	link := conn.NewLink(s.task.Client.GetId(), common.CONN_TCP, addr, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, c, s.task.Flow, nil, s.task.Client.Rate, nil)

	if tunnel, err := s.bridge.SendLinkInfo(s.task.Client.Id, link, c.Conn.RemoteAddr().String()); err != nil {
		c.Close()
		return err
	} else {
		link.Run(true)
		s.linkCopy(link, c, rb, tunnel, s.task.Flow)
	}
	return nil
}

//close
func (s *TunnelModeServer) Close() error {
	return s.listener.Close()
}

//web管理方式
type WebServer struct {
	server
}

//开始
func (s *WebServer) Start() error {
	p, _ := beego.AppConfig.Int("httpport")
	if !common.TestTcpPort(p) {
		lg.Fatalf("Web management port %d is occupied", p)
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	lg.Println("Web management start, access port is", p)
	beego.SetStaticPath("/static", filepath.Join(common.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(common.GetRunPath(), "web", "views"))
	beego.Run()
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
	return s.dealClient(c, s.task.Client.Cnf, s.task.Target, "", nil)
}

//http代理模式
func ProcessHttp(c *conn.Conn, s *TunnelModeServer) error {
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		lg.Println(err)
		return err
	}
	if r.Method == "CONNECT" {
		c.Write([]byte("HTTP/1.1 200 Connection Established\r\n"))
		rb = nil //reset
	}
	if err := s.auth(r, c, s.task.Client.Cnf.U, s.task.Client.Cnf.P); err != nil {
		return err
	}
	return s.dealClient(c, s.task.Client.Cnf, addr, method, rb)
}
