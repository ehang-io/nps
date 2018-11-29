package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type TRPClient struct {
	svrAddr string
	tcpNum  int
	sync.Mutex
}

func NewRPClient(svraddr string, tcpNum int) *TRPClient {
	c := new(TRPClient)
	c.svrAddr = svraddr
	c.tcpNum = tcpNum
	return c
}

func (s *TRPClient) Start() error {
	for i := 0; i < s.tcpNum; i++ {
		go s.newConn()
	}
	for {
		time.Sleep(time.Second * 5)
	}
	return nil
}

//新建
func (s *TRPClient) newConn() error {
	s.Lock()
	conn, err := net.Dial("tcp", s.svrAddr)
	if err != nil {
		log.Println("连接服务端失败,五秒后将重连")
		time.Sleep(time.Second * 5)
		s.Unlock()
		go s.newConn()
		return err
	}
	s.Unlock()
	return s.process(NewConn(conn))
}

func (s *TRPClient) process(c *Conn) error {
	c.SetAlive()
	if _, err := c.Write(getverifyval()); err != nil {
		return err
	}
	c.wMain()
	for {
		flags, err := c.ReadFlag()
		if err != nil {
			log.Println("服务端断开,五秒后将重连", err)
			time.Sleep(5 * time.Second)
			go s.newConn()
			break
		}
		switch flags {
		case VERIFY_EER:
			log.Fatal("vkey不正确,请检查配置文件")
		case RES_SIGN: //代理请求模式
			if err := s.dealHttp(c); err != nil {
				log.Println(err)
				return err
			}
		case WORK_CHAN: //隧道模式，每次开启10个，加快连接速度
			for i := 0; i < 10; i++ {
				go s.dealChan()
			}
		case RES_MSG:
			log.Println("服务端返回错误。")
		default:
			log.Println("无法解析该错误。")
		}
	}
	return nil
}

//隧道模式处理
func (s *TRPClient) dealChan() error {
	//创建一个tcp连接
	conn, err := net.Dial("tcp", s.svrAddr)
	//验证
	if _, err := conn.Write(getverifyval()); err != nil {
		return err
	}
	//默认长连接保持
	c := NewConn(conn)
	c.SetAlive()
	//写标志
	c.wChan()
	//获取连接的host
	host, err := c.GetHostFromConn()
	if err != nil {
		return err
	}
	//与目标建立连接
	server, err := net.Dial("tcp", host)
	if err != nil {
		return err
	}
	//创建成功后io.copy
	go relay(server, c.conn)
	relay(c.conn, server)
	return nil
}

//http模式处理
func (s *TRPClient) dealHttp(c *Conn) error {
	nlen, err := c.GetLen()
	if err != nil {
		c.wError()
		return err
	}
	raw, err := c.ReadLen(int(nlen))
	if err != nil {
		c.wError()
		return err
	}
	req, err := DecodeRequest(raw)
	if err != nil {
		c.wError()
		return err
	}
	respBytes, err := GetEncodeResponse(req)
	if err != nil {
		c.wError()
		return err
	}
	n, err := c.Write(respBytes)
	if err != nil {
		return err
	}
	if n != len(respBytes) {
		return errors.New(fmt.Sprintf("发送数据长度错误，已经发送：%dbyte，总字节长：%dbyte\n", n, len(respBytes)))
	}
	return nil
}
