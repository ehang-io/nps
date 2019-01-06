package lib

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/session"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
)

var GlobalHostSessions *session.Manager

const (
	VERIFY_EER         = "vkey"
	WORK_MAIN          = "main"
	WORK_CHAN          = "chan"
	RES_SIGN           = "sign"
	RES_MSG            = "msg0"
	CONN_SUCCESS       = "sucs"
	CONN_ERROR         = "fail"
	TEST_FLAG          = "tst"
	CONN_TCP           = "tcp"
	CONN_UDP           = "udp"
	Unauthorized_BYTES = `HTTP/1.1 401 Unauthorized
Content-Type: text/plain; charset=utf-8
WWW-Authenticate: Basic realm="easyProxy"

401 Unauthorized`
)

type process func(c *Conn, s *TunnelModeServer) error

type HttpModeServer struct {
	bridge *Tunnel
	config *ServerConfig
}

//http
func NewHttpModeServer(bridge *Tunnel, cnf *ServerConfig) *HttpModeServer {
	s := new(HttpModeServer)
	s.bridge = bridge
	s.config = cnf
	return s
}

//开始
func (s *HttpModeServer) Start() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	retry:
		u := beego.AppConfig.String("basic.user")
		p := beego.AppConfig.String("basic.password")
		if u != "" && p != "" && !checkAuth(r, u, p) {
			w.Header().Set("WWW-Authenticate", `Basic realm="easyProxy""`)
			w.WriteHeader(401)
			w.Write([]byte("401 Unauthorized\n"))
			return
		}
		err, conn := s.bridge.GetSignal(getverifyval(s.config.VerifyKey))
		if err != nil {
			BadRequest(w)
			return
		}
		if err := s.writeRequest(r, conn); err != nil {
			log.Println("write request to client error:", err)
			conn.Close()
			goto retry
			return
		}
		err = s.writeResponse(w, conn)
		if err != nil {
			log.Println("write response error:", err)
			conn.Close()
			goto retry
			return
		}
		s.bridge.ReturnSignal(conn, getverifyval(s.config.VerifyKey))
	})
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.config.TcpPort), nil))
}

//req转为bytes发送给client端
func (s *HttpModeServer) writeRequest(r *http.Request, conn *Conn) error {
	raw, err := EncodeRequest(r)
	if err != nil {
		return err
	}
	conn.wSign()
	conn.WriteConnInfo(s.config.CompressEncode, s.config.CompressDecode, s.config.Crypt, s.config.Mux)
	c, err := conn.WriteTo(raw, s.config.CompressEncode, s.config.Crypt)
	if err != nil {
		return err
	}
	if c != len(raw) {
		return errors.New("写出长度与字节长度不一致。")
	}
	return nil
}

//从client读取出Response
func (s *HttpModeServer) writeResponse(w http.ResponseWriter, c *Conn) error {
	flags, err := c.ReadFlag()
	if err != nil {
		return err
	}
	switch flags {
	case RES_SIGN:
		buf := make([]byte, 1024*1024*32)
		n, err := c.ReadFrom(buf, s.config.CompressDecode, s.config.Crypt)
		if err != nil {
			return err
		}
		resp, err := DecodeResponse(buf[:n])
		if err != nil {
			return err
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		for k, v := range resp.Header {
			for _, v2 := range v {
				w.Header().Set(k, v2)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(bodyBytes)
	case RES_MSG:
		BadRequest(w)
		return errors.New("客户端请求出错")
	default:
		BadRequest(w)
		return errors.New("无法解析此错误")
	}
	return nil
}

type TunnelModeServer struct {
	process  process
	bridge   *Tunnel
	config   *ServerConfig
	listener *net.TCPListener
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge *Tunnel, cnf *ServerConfig) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.config = cnf
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
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
		go s.process(NewConn(conn), s)
	}
	return nil
}

//权限认证
func (s *TunnelModeServer) auth(r *http.Request, c *Conn, u, p string) error {
	if u != "" && p != "" && !checkAuth(r, u, p) {
		c.Write([]byte(Unauthorized_BYTES))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}

//与客户端建立通道
func (s *TunnelModeServer) dealClient(c *Conn, cnf *ServerConfig, addr string, method string, rb []byte) error {
	link, err := s.bridge.GetTunnel(getverifyval(cnf.VerifyKey), cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
	defer func() {
		if cnf.Mux {
			s.bridge.ReturnTunnel(link, getverifyval(cnf.VerifyKey))
		}
	}()
	if err != nil {
		log.Println(err)
		c.Close()
		return err
	}
	if _, err := link.WriteHost(CONN_TCP, addr); err != nil {
		c.Close()
		link.Close()
		log.Println(err)
		return err
	}
	if flag, err := link.ReadFlag(); err == nil {
		if flag == CONN_SUCCESS {
			if method == "CONNECT" {
				fmt.Fprint(c, "HTTP/1.1 200 Connection established\r\n")
			} else {
				link.WriteTo(rb, cnf.CompressEncode, cnf.Crypt)
			}
			go relay(link.conn, c.conn, cnf.CompressEncode, cnf.Crypt, cnf.Mux)
			relay(c.conn, link.conn, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
		} else {
			c.Close()
		}
	}
	return nil
}

//close
func (s *TunnelModeServer) Close() error {
	return s.listener.Close()
}

//tcp隧道模式
func ProcessTunnel(c *Conn, s *TunnelModeServer) error {
	method, _, rb, err, r := c.GetHost()
	if err == nil {
		if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
			return err
		}
	}
	return s.dealClient(c, s.config, s.config.Target, method, rb)
}

//http代理模式
func ProcessHttp(c *Conn, s *TunnelModeServer) error {
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		return err
	}
	if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
		return err
	}
	//TODO效率问题
	return s.dealClient(c, s.config, addr, method, rb)
}

//多客户端域名代理
func ProcessHost(c *Conn, s *TunnelModeServer) error {
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		return err
	}
	host, task, err := getKeyByHost(addr)
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
	bridge *Tunnel
}

//开始
func (s *WebServer) Start() {
	InitFromCsv()
	p, _ := beego.AppConfig.Int("hostPort")
	t := &ServerConfig{
		TcpPort:      p,
		Mode:         "httpHostServer",
		Target:       "",
		VerifyKey:    "",
		U:            "",
		P:            "",
		Compress:     "",
		Start:        1,
		IsRun:        0,
		ClientStatus: 0,
	}
	AddTask(t)
	beego.BConfig.WebConfig.Session.SessionOn = true
	log.Println("web管理启动，访问端口为", beego.AppConfig.String("httpport"))
	beego.SetViewsPath(beego.AppPath + "/views/")
	beego.SetStaticPath("/static/", beego.AppPath+"/static/")
	beego.Run()
}

//new
func NewWebServer(bridge *Tunnel) *WebServer {
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

//TODO：host模式的客户端，无需指定和监听端口等
func NewHostServer(cnf *ServerConfig) *HostServer {
	s := new(HostServer)
	s.config = cnf
	return s
}

//close
func (s *HostServer) Close() error {
	return nil
}
