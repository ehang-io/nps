package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type TRPServer struct {
	tcpPort  int
	httpPort int
	listener *net.TCPListener
	connList chan net.Conn
	sync.RWMutex
}

func NewRPServer(tcpPort, httpPort int) *TRPServer {
	s := new(TRPServer)
	s.tcpPort = tcpPort
	s.httpPort = httpPort
	s.connList = make(chan net.Conn, 1000)
	return s
}

func (s *TRPServer) Start() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.tcpPort, ""})
	if err != nil {
		return err
	}
	go s.httpserver()
	return s.tcpserver()
}

func (s *TRPServer) Close() error {
	if s.listener != nil {
		err := s.listener.Close()
		s.listener = nil
		return err
	}
	return errors.New("TCP实例未创建！")
}

func (s *TRPServer) tcpserver() error {
	var err error
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.cliProcess(conn)
	}
	return err
}

func badRequest(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

func (s *TRPServer) httpserver() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	retry:
		if len(s.connList) == 0 {
			badRequest(w)
			return
		}
		conn := <-s.connList
		log.Println(r.RequestURI)
		err := s.write(r, conn)
		if err != nil {
			log.Println(err)
			conn.Close()
			goto retry
			return
		}
		err = s.read(w, conn)
		if err != nil {
			log.Println(err)
			conn.Close()
			goto retry
			return
		}
		s.connList <- conn
		conn = nil
	})
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.httpPort), nil))
}

func (s *TRPServer) cliProcess(conn *net.TCPConn) error {
	conn.SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	vval := make([]byte, 20)
	_, err := conn.Read(vval)
	if err != nil {
		log.Println("客户端读超时。客户端地址为：:", conn.RemoteAddr())
		conn.Close()
		return err
	}
	if bytes.Compare(vval, getverifyval()[:]) != 0 {
		log.Println("当前客户端连接校验错误，关闭此客户端:", conn.RemoteAddr())
		conn.Write([]byte("vkey"))
		conn.Close()
		return err
	}
	conn.SetReadDeadline(time.Time{})
	log.Println("连接新的客户端：", conn.RemoteAddr())
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Duration(2 * time.Second))
	s.connList <- conn
	return nil
}

func (s *TRPServer) write(r *http.Request, conn net.Conn) error {
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

func (s *TRPServer) read(w http.ResponseWriter, conn net.Conn) (error) {
	val := make([]byte, 4)
	_, err := conn.Read(val)
	if err != nil {
		return err
	}
	flags := string(val)
	switch flags {
	case "sign":
		_, err = conn.Read(val)
		if err != nil {
			return err
		}
		nlen := int(binary.LittleEndian.Uint32(val))
		if nlen == 0 {
			return errors.New("读取客户端长度错误。")
		}
		log.Println("收到客户端数据，需要读取长度：", nlen)
		raw := make([]byte, 0)
		buff := make([]byte, 1024)
		c := 0
		for {
			clen, err := conn.Read(buff)
			if err != nil && err != io.EOF {
				return err
			}
			raw = append(raw, buff[:clen]...)
			c += clen
			if c >= nlen {
				break
			}
		}
		log.Println("读取完成，长度：", c, "实际raw长度：", len(raw))
		if c != nlen {
			return fmt.Errorf("已读取长度错误，已读取%dbyte，需要读取%dbyte。", c, nlen)
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
	case "msg0":
		return nil
	default:
		log.Println("无法解析此错误", string(val))
	}
	return nil
}
