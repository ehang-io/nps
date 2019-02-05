package server

import (
	"errors"
	"github.com/astaxie/beego"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib"
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
func NewTunnelModeServer(process process, bridge *bridge.Bridge, task *lib.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	s.config = lib.DeepCopyConfig(task.Config)
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.task.TcpPort, ""})
	if err != nil {
		return err
	}
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			lib.Println(err)
			continue
		}
		go s.process(lib.NewConn(conn), s)
	}
	return nil
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *lib.Conn, cnf *lib.Config, addr string, method string, rb []byte) error {
	link := lib.NewLink(s.task.Client.GetId(), lib.CONN_TCP, addr, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, c, s.task.Flow, nil, s.task.Client.Rate, nil)

	if tunnel, err := s.bridge.SendLinkInfo(s.task.Client.Id, link); err != nil {
		c.Close()
		return err
	} else {
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
	if !lib.TestTcpPort(p) {
		lib.Fatalln("web管理端口", p, "被占用!")
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	lib.Println("web管理启动，访问端口为", beego.AppConfig.String("httpport"))
	beego.SetStaticPath("/static", filepath.Join(lib.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(lib.GetRunPath(), "web", "views"))
	beego.Run()
	return errors.New("web管理启动失败")
}

//new
func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

//host
type HostServer struct {
	server
}

//开始
func (s *HostServer) Start() error {
	return nil
}

func NewHostServer(task *lib.Tunnel) *HostServer {
	s := new(HostServer)
	s.task = task
	s.config = lib.DeepCopyConfig(task.Config)
	return s
}

//close
func (s *HostServer) Close() error {
	return nil
}

type process func(c *lib.Conn, s *TunnelModeServer) error

//tcp隧道模式
func ProcessTunnel(c *lib.Conn, s *TunnelModeServer) error {
	if !s.ResetConfig() {
		c.Close()
		return errors.New("流量超出")
	}
	return s.dealClient(c, s.config, s.task.Target, "", nil)
}

//http代理模式
func ProcessHttp(c *lib.Conn, s *TunnelModeServer) error {
	if !s.ResetConfig() {
		c.Close()
		return errors.New("流量超出")
	}
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		return err
	}
	if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
		return err
	}
	return s.dealClient(c, s.config, addr, method, rb)
}
