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

type TunnelModeServer struct {
	server
	errorContent []byte
	process      process
	listener     *net.TCPListener
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge *bridge.Tunnel, cnf *utils.ServerConfig) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.config = cnf
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	var err error
	if s.errorContent, err = utils.ReadAllFromFile(beego.AppPath + "/web/static/page/error.html"); err != nil {
		s.errorContent = []byte("easyProxy 404")
	}
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
		s.ResetConfig()
		go s.process(utils.NewConn(conn), s)
	}
	return nil
}

//权限认证
func (s *TunnelModeServer) auth(r *http.Request, c *utils.Conn, u, p string) error {
	if u != "" && p != "" && !utils.CheckAuth(r, u, p) {
		c.Write([]byte(utils.UnauthorizedBytes))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}

func (s *TunnelModeServer) writeConnFail(c net.Conn) {
	c.Write([]byte(utils.ConnectionFailBytes))
	c.Write(s.errorContent)
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *utils.Conn, cnf *utils.ServerConfig, addr string, method string, rb []byte) error {
	var link *utils.Conn
	var err error
	defer func() {
		if cnf.Mux && link != nil {
			s.bridge.ReturnTunnel(link, cnf.ClientId)
		}
	}()
	if link, err = s.GetTunnelAndWriteHost(utils.CONN_TCP, cnf, addr); err != nil {
		log.Println("get bridge tunnel error: ", err)
		return err
	}
	if flag, err := link.ReadFlag(); err == nil {
		if flag == utils.CONN_SUCCESS {
			if method == "CONNECT" {
				fmt.Fprint(c, "HTTP/1.1 200 Connection established\r\n")
			} else if rb != nil {
				link.WriteTo(rb, cnf.CompressEncode, cnf.Crypt)
			}
			out, in := utils.ReplayWaitGroup(link.Conn, c.Conn, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
			s.FlowAdd(in, out)
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
func NewWebServer(bridge *bridge.Tunnel) *WebServer {
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

func NewHostServer(cnf *utils.ServerConfig) *HostServer {
	s := new(HostServer)
	s.config = cnf
	return s
}

//close
func (s *HostServer) Close() error {
	return nil
}
