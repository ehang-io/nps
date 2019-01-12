package server

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
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

func (s *TunnelModeServer) dealClient2(c *utils.Conn, cnf *ServerConfig, addr string, method string, rb []byte) error {
	return nil
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *utils.Conn, cnf *ServerConfig, addr string, method string, rb []byte) error {
	var link *utils.Conn
	var err error
	defer func() {
		if cnf.Mux && link != nil {
			s.bridge.ReturnTunnel(link, getverifyval(cnf.VerifyKey))
		}
	}()
	if link, err = s.GetTunnelAndWriteHost(c, cnf, addr); err != nil {
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
			utils.ReplayWaitGroup(link.Conn, c.Conn, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
		}
	}
	return nil
}

//close
func (s *TunnelModeServer) Close() error {
	return s.listener.Close()
}

func (s *TunnelModeServer) GetTunnelAndWriteHost(c *utils.Conn, cnf *ServerConfig, addr string) (*utils.Conn, error) {
	var err error
	link, err := s.bridge.GetTunnel(getverifyval(cnf.VerifyKey), cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
	if err != nil {
		return nil, err
	}
	if _, err = link.WriteHost(utils.CONN_TCP, addr); err != nil {
		link.Close()
		return nil, err
	}
	return link, nil
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
	return s.dealClient(c, s.config, addr, method, rb)
}

//多客户端域名代理
func ProcessHost(c *utils.Conn, s *TunnelModeServer) error {
	var (
		isConn = true
		link   *utils.Conn
		cnf    *ServerConfig
		host   *HostList
		wg     sync.WaitGroup
	)
	for {
		r, err := http.ReadRequest(bufio.NewReader(c))
		if err != nil {
			break
		}
		//首次获取conn
		if isConn {
			isConn = false
			if host, cnf, err = GetKeyByHost(r.Host); err != nil {
				log.Printf("the host %s is not found !", r.Host)
				break
			}

			if err = s.auth(r, c, cnf.U, cnf.P); err != nil {
				break
			}

			if link, err = s.GetTunnelAndWriteHost(c, cnf, host.Target); err != nil {
				log.Println("get bridge tunnel error: ", err)
				break
			}

			if flag, err := link.ReadFlag(); err != nil || flag == utils.CONN_ERROR {
				log.Printf("the host %s connection to %s error", r.Host, host.Target)
				break
			} else {
				wg.Add(1)
				go func() {
					utils.Relay(c.Conn, link.Conn, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
					wg.Done()
				}()
			}
		}
		utils.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			break
		}
		if _, err := link.WriteTo(b, cnf.CompressEncode, cnf.Crypt); err != nil {
			break
		}
	}
	wg.Wait()
	if cnf != nil && cnf.Mux && link != nil {
		link.WriteTo([]byte(utils.IO_EOF), cnf.CompressEncode, cnf.Crypt)
		s.bridge.ReturnTunnel(link, getverifyval(cnf.VerifyKey))
	} else if link != nil {
		link.Close()
	}
	c.Close()
	return nil
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
