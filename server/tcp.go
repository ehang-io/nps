package server

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"net/http"
	"strings"
)

type process func(c *utils.Conn, s *TunnelModeServer) error

type TunnelModeServer struct {
	process  process
	bridge   *bridge.Tunnel
	config   *ServerConfig
	listener *net.TCPListener
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge *bridge.Tunnel, cnf *ServerConfig) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.config = cnf
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.config.TcpPort, ""})
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

//权限认证
func (s *TunnelModeServer) auth(r *http.Request, c *utils.Conn, u, p string) error {
	if u != "" && p != "" && !utils.CheckAuth(r, u, p) {
		c.Write([]byte(utils.Unauthorized_BYTES))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *utils.Conn, cnf *ServerConfig, addr string, method string, rb []byte) error {
reGet:
	link, err := s.bridge.GetTunnel(getverifyval(cnf.VerifyKey), cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
	defer func() {
		if cnf.Mux {
			s.bridge.ReturnTunnel(link, getverifyval(cnf.VerifyKey))
		} else {
			c.Close()
		}
	}()
	if err != nil {
		log.Println("conn to client error:", err)
		c.Close()
		return err
	}
	if _, err := link.WriteHost(utils.CONN_TCP, addr); err != nil {
		c.Close()
		link.Close()
		log.Println(err)
		goto reGet
	}
	if flag, err := link.ReadFlag(); err == nil {
		if flag == utils.CONN_SUCCESS {
			if method == "CONNECT" {
				fmt.Fprint(c, "HTTP/1.1 200 Connection established\r\n")
			} else if rb != nil {
				link.WriteTo(rb, cnf.CompressEncode, cnf.Crypt)
			}
			go utils.Relay(link.Conn, c.Conn, cnf.CompressEncode, cnf.Crypt, cnf.Mux)
			utils.Relay(c.Conn, link.Conn, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
		}
	}
	return nil
}

//close
func (s *TunnelModeServer) Close() error {
	return s.listener.Close()
}

//tcp隧道模式
func ProcessTunnel(c *utils.Conn, s *TunnelModeServer) error {
	_, _, rb, err, r := c.GetHost()
	if err == nil {
		if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
			return err
		}
	}
	return s.dealClient(c, s.config, s.config.Target, "", rb)
}

//http代理模式
func ProcessHttp(c *utils.Conn, s *TunnelModeServer) error {
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		log.Println(err)
		c.Close()
		return err
	}
	if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
		return err
	}
	//TODO 效率问题
	return s.dealClient(c, s.config, addr, method, rb)
}

//多客户端域名代理
func ProcessHost(c *utils.Conn, s *TunnelModeServer) error {
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		return err
	}
	host, task, err := GetKeyByHost(addr)
	if err != nil {
		return err
	}
	if err := s.auth(r, c, task.U, task.P); err != nil {
		return err
	}
	if err != nil {
		c.Close()
		return err
	}
	return s.dealClient(c, task, host.Target, method, rb)
}

//web管理方式
type WebServer struct {
	bridge *bridge.Tunnel
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
func NewWebServer(bridge *bridge.Tunnel) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

//host
type HostServer struct {
	config *ServerConfig
}

//开始
func (s *HostServer) Start() error {
	return nil
}

func NewHostServer(cnf *ServerConfig) *HostServer {
	s := new(HostServer)
	s.config = cnf
	return s
}

//close
func (s *HostServer) Close() error {
	return nil
}
