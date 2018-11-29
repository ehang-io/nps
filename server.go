package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

const (
	VERIFY_EER = "vkey"
	WORK_MAIN  = "main"
	WORK_CHAN  = "chan"
	RES_SIGN   = "sign"
	RES_MSG    = "msg0"
)

type HttpModeServer struct {
	Tunnel
	httpPort int
}

func NewHttpModeServer(tcpPort, httpPort int) *HttpModeServer {
	s := new(HttpModeServer)
	s.tunnelPort = tcpPort
	s.httpPort = httpPort
	s.signalList = make(chan *Conn, 1000)
	return s
}

//开始
func (s *HttpModeServer) Start() (error) {
	err := s.StartTunnel()
	if err != nil {
		log.Fatalln("开启客户端失败!", err)
		return err
	}
	s.startHttpServer()
	return nil
}

//开启http端口监听
func (s *HttpModeServer) startHttpServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	retry:
		if len(s.signalList) == 0 {
			BadRequest(w)
			return
		}
		conn := <-s.signalList
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
		s.signalList <- conn
	})
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.httpPort), nil))
}

//req转为bytes发送给client端
func (s *HttpModeServer) writeRequest(r *http.Request, conn *Conn) error {
	raw, err := EncodeRequest(r)
	if err != nil {
		return err
	}
	c, err := conn.Write(raw)
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
		nlen, err := c.GetLen()
		if err != nil {
			return err
		}
		raw, err := c.ReadLen(nlen)
		if err != nil {
			return err
		}
		resp, err := DecodeResponse(raw)
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
	Tunnel
	httpPort     int
	tunnelTarget string
}

func NewTunnelModeServer(tcpPort, httpPort int, tunnelTarget string) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.tunnelPort = tcpPort
	s.httpPort = httpPort
	s.tunnelTarget = tunnelTarget
	s.tunnelList = make(chan *Conn, 1000)
	s.signalList = make(chan *Conn, 10)
	return s
}

//开始
func (s *TunnelModeServer) Start() (error) {
	err := s.StartTunnel()
	if err != nil {
		log.Fatalln("开启客户端失败!", err)
		return err
	}
	s.startTunnelServer()
	return nil
}

//隧道模式server
func (s *TunnelModeServer) startTunnelServer() {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.httpPort, ""})
	if err != nil {
		log.Fatalln(err)
	}
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.process(NewConn(conn))
	}
}

//监听连接处理
func (s *TunnelModeServer) process(c *Conn) error {
retry:
	if len(s.tunnelList) < 10 { //新建通道
		go s.newChan()
	}
	link := <-s.tunnelList
	if _, err := link.WriteHost(s.tunnelTarget); err != nil {
		goto retry
	}
	go io.Copy(link, c)
	io.Copy(c, link.conn)
	return nil
}
