package server

import (
	"errors"
	"github.com/astaxie/beego"
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"strings"
)

type TunnelModeServer struct {
	server
	process  process
	listener *net.TCPListener
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge *bridge.Bridge, task *utils.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	s.config = utils.DeepCopyConfig(task.Config)
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
			log.Println(err)
			continue
		}
		go s.process(utils.NewConn(conn), s)
	}
	return nil
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *utils.Conn, cnf *utils.Config, addr string, method string, rb []byte) error {
	link := utils.NewLink(s.task.Client.GetId(), utils.CONN_TCP, addr, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, c, s.task.Flow, nil, s.task.Client.Rate, nil)

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
func (s *WebServer) Start() {
	beego.BConfig.WebConfig.Session.SessionOn = true
	log.Println("web管理启动，访问端口为", beego.AppConfig.String("httpport"))
	beego.SetViewsPath(beego.AppPath + "/web/views")
	beego.SetStaticPath("/static", beego.AppPath+"/web/static")
	beego.Run()
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

func NewHostServer(task *utils.Tunnel) *HostServer {
	s := new(HostServer)
	s.task = task
	s.config = utils.DeepCopyConfig(task.Config)
	return s
}

//close
func (s *HostServer) Close() error {
	return nil
}

type process func(c *utils.Conn, s *TunnelModeServer) error

//tcp隧道模式
func ProcessTunnel(c *utils.Conn, s *TunnelModeServer) error {
	if !s.ResetConfig() {
		c.Close()
		return errors.New("流量超出")
	}
	return s.dealClient(c, s.config, s.task.Target, "", nil)
}

//http代理模式
func ProcessHttp(c *utils.Conn, s *TunnelModeServer) error {
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
