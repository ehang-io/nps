package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type Tunnel struct {
	tunnelPort int              //通信隧道端口
	listener   *net.TCPListener //server端监听
	signalList chan *Conn       //通信
	tunnelList chan *Conn       //隧道
	sync.RWMutex
}

func (s *Tunnel) StartTunnel() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.tunnelPort, ""})
	if err != nil {
		return err
	}
	go s.tunnelProcess()
	return nil
}

//tcp server
func (s *Tunnel) tunnelProcess() error {
	var err error
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.cliProcess(NewConn(conn))
	}
	return err
}

//验证失败，返回错误验证flag，并且关闭连接
func (s *Tunnel) verifyError(c *Conn) {
	c.conn.Write([]byte(VERIFY_EER))
	c.conn.Close()
}

func (s *Tunnel) cliProcess(c *Conn) error {
	c.conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	vval := make([]byte, 20)
	_, err := c.conn.Read(vval)
	if err != nil {
		log.Println("客户端读超时。客户端地址为：:", c.conn.RemoteAddr())
		c.conn.Close()
		return err
	}
	if bytes.Compare(vval, getverifyval()[:]) != 0 {
		log.Println("当前客户端连接校验错误，关闭此客户端:", c.conn.RemoteAddr())
		s.verifyError(c)
		return err
	}
	c.conn.(*net.TCPConn).SetReadDeadline(time.Time{})
	//做一个判断 添加到对应的channel里面以供使用
	flag, err := c.ReadFlag()
	if err != nil {
		return err
	}
	return s.typeDeal(flag, c)
}

//tcp连接类型区分
func (s *Tunnel) typeDeal(typeVal string, c *Conn) error {
	switch typeVal {
	case WORK_MAIN:
		s.signalList <- c
	case WORK_CHAN:
		s.tunnelList <- c
	default:
		return errors.New("无法识别")
	}
	c.SetAlive()
	return nil
}

//新建隧道
func (s *Tunnel) newChan() {
retry:
	connPass := <-s.signalList
	_, err := connPass.conn.Write([]byte("chan"))
	if err != nil {
		fmt.Println(err)
		goto retry
	}
	s.signalList <- connPass
}
