package lib

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
)

const (
	VERIFY_EER = "vkey"
	WORK_MAIN  = "main"
	WORK_CHAN  = "chan"
	RES_SIGN   = "sign"
	RES_MSG    = "msg0"
	TEST_FLAG  = "tst"
	CONN_TCP   = "tcp"
	CONN_UDP   = "udp"
)

type HttpModeServer struct {
	bridge     *Tunnel
	httpPort   int
	enCompress int
	deCompress int
	vKey       string
}

func NewHttpModeServer(httpPort int, bridge *Tunnel, enCompress int, deCompress int, vKey string) *HttpModeServer {
	s := new(HttpModeServer)
	s.bridge = bridge
	s.httpPort = httpPort
	s.enCompress = enCompress
	s.deCompress = deCompress
	s.vKey = vKey
	return s
}

//开始
func (s *HttpModeServer) Start() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	retry:
		err, conn := s.bridge.GetSignal(getverifyval(s.vKey))
		if err != nil {
			BadRequest(w)
		}
		if err := s.writeRequest(r, conn); err != nil {
			log.Println(err)
			conn.Close()
			goto retry
			return
		}
		err = s.writeResponse(w, conn)
		if err != nil {
			log.Println(err)
			conn.Close()
			goto retry
			return
		}
		s.bridge.ReturnSignal(conn, getverifyval(s.vKey))
	})
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.httpPort), nil))
}

//req转为bytes发送给client端
func (s *HttpModeServer) writeRequest(r *http.Request, conn *Conn) error {
	raw, err := EncodeRequest(r)
	if err != nil {
		return err
	}
	conn.wSign()
	conn.WriteCompressType(s.enCompress, s.deCompress)
	c, err := conn.WriteCompress(raw, s.enCompress)
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
		buf := make([]byte, 1024*32)
		n, err := c.ReadFromCompress(buf, s.deCompress)
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

type process func(c *Conn, s *TunnelModeServer) error

type TunnelModeServer struct {
	httpPort     int
	tunnelTarget string
	process      process
	bridge       *Tunnel
	listener     *net.TCPListener
	enCompress   int
	deCompress   int
	vKey         string
}

func NewTunnelModeServer(httpPort int, tunnelTarget string, process process, bridge *Tunnel, enCompress int, deCompress int, vKey string) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.httpPort = httpPort
	s.bridge = bridge
	s.tunnelTarget = tunnelTarget
	s.process = process
	s.enCompress = enCompress
	s.deCompress = deCompress
	s.vKey = vKey
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.httpPort, ""})
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

func (s *TunnelModeServer) Close() error {
	return s.listener.Close()
}

//tcp隧道模式
func ProcessTunnel(c *Conn, s *TunnelModeServer) error {
	link := s.bridge.GetTunnel(getverifyval(s.vKey), s.enCompress, s.deCompress)
	if _, err := link.WriteHost(CONN_TCP, s.tunnelTarget); err != nil {
		link.Close()
		c.Close()
		log.Println(err)
		return err
	}
	go relay(link, c, s.enCompress)
	relay(c, link, s.deCompress)
	return nil
}

//http代理模式
func ProcessHttp(c *Conn, s *TunnelModeServer) error {
	method, addr, rb, err := c.GetHost()
	if err != nil {
		c.Close()
		return err
	}
	link := s.bridge.GetTunnel(getverifyval(s.vKey), s.enCompress, s.deCompress)
	if _, err := link.WriteHost(CONN_TCP, addr); err != nil {
		c.Close()
		link.Close()
		log.Println(err)
		return err
	}
	if method == "CONNECT" {
		fmt.Fprint(c, "HTTP/1.1 200 Connection established\r\n")
	} else {
		link.WriteCompress(rb, s.enCompress)
	}
	go relay(link, c, s.enCompress)
	relay(c, link, s.deCompress)
	return nil
}

//多客户端域名代理
func ProcessHost(c *Conn, s *TunnelModeServer) error {
	method, addr, rb, err := c.GetHost()
	if err != nil {
		c.Close()
		return err
	}
	host, task, err := getKeyByHost(addr)
	if err != nil {
		c.Close()
		return err
	}
	de, en := getCompressType(task.Compress)
	link := s.bridge.GetTunnel(getverifyval(host.Vkey), en, de)
	if _, err := link.WriteHost(CONN_TCP, host.Target); err != nil {
		c.Close()
		link.Close()
		log.Println(err)
		return err
	}
	if method == "CONNECT" {
		fmt.Fprint(c, "HTTP/1.1 200 Connection established\r\n")
	} else {
		link.WriteCompress(rb, en)
	}
	go relay(link, c, en)
	relay(c, link, de)
	return nil
}

//web管理方式
type WebServer struct {
	bridge *Tunnel
}

//开始
func (s *WebServer) Start() {
	InitFromCsv()
	p, _ := beego.AppConfig.Int("hostPort")
	t := &TaskList{
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
	log.Println("web管理启动，访问端口为",beego.AppConfig.String("httpport"))
	beego.Run()
}

func NewWebServer(bridge *Tunnel) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

//host
type HostServer struct {
}

//开始
func (s *HostServer) Start() error {
	return nil
}
func NewHostServer() *HostServer {
	s := new(HostServer)
	return s
}

func (s *HostServer) Close() error {
	return nil
}
