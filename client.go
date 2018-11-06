package main

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	disabledRedirect = errors.New("disabled redirect.")
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

func (c *TRPClient) Start() error {
	for i := 0; i < c.tcpNum; i++ {
		go c.newConn()
	}
	for {
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (c *TRPClient) newConn() error {
	c.Lock()
	conn, err := net.Dial("tcp", c.svrAddr)
	if err != nil {
		log.Println("连接服务端失败,五秒后将重连")
		time.Sleep(time.Second * 5)
		c.Unlock()
		c.newConn()
		return err
	}
	c.Unlock()
	conn.(*net.TCPConn).SetKeepAlive(true)
	conn.(*net.TCPConn).SetKeepAlivePeriod(time.Duration(2 * time.Second))
	return c.process(conn)
}

func (c *TRPClient) werror(conn net.Conn) {
	conn.Write([]byte("msg0"))
}

func (c *TRPClient) process(conn net.Conn) error {
	if _, err := conn.Write(getverifyval()); err != nil {
		return err
	}
	val := make([]byte, 4)
	for {
		_, err := conn.Read(val)
		if err != nil {
			log.Println("服务端断开,五秒后将重连", err)
			time.Sleep(5 * time.Second)
			go c.newConn()
			return err
		}
		flags := string(val)
		switch flags {
		case "vkey":
			log.Fatal("vkey不正确,请检查配置文件")
		case "sign":
			c.deal(conn)
		case "msg0":
			log.Println("服务端返回错误。")
		default:
			log.Println("无法解析该错误。")
		}
	}
	return nil
}
func (c *TRPClient) deal(conn net.Conn) error {
	val := make([]byte, 4)
	_, err := conn.Read(val)
	nlen := binary.LittleEndian.Uint32(val)
	log.Println("收到服务端数据，长度：", nlen)
	if nlen <= 0 {
		log.Println("数据长度错误。")
		c.werror(conn)
		return errors.New("数据长度错误")
	}
	raw := make([]byte, nlen)
	n, err := conn.Read(raw)
	if err != nil {
		return err
	}
	if n != int(nlen) {
		log.Printf("读取服务端数据长度错误，已经读取%dbyte，总长度%d字节\n", n, nlen)
		c.werror(conn)
		return errors.New("读取服务端数据长度错误")
	}
	req, err := DecodeRequest(raw)
	if err != nil {
		log.Println("DecodeRequest错误：", err)
		c.werror(conn)
		return err
	}
	rawQuery := ""
	if req.URL.RawQuery != "" {
		rawQuery = "?" + req.URL.RawQuery
	}
	log.Println(req.URL.Path + rawQuery)
	client := new(http.Client)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return disabledRedirect
	}
	resp, err := client.Do(req)
	disRedirect := err != nil && strings.Contains(err.Error(), disabledRedirect.Error())
	if err != nil && !disRedirect {
		log.Println("请求本地客户端错误：", err)
		c.werror(conn)
		return err
	}
	if !disRedirect {
		defer resp.Body.Close()
	} else {
		resp.Body = nil
		resp.ContentLength = 0
	}
	respBytes, err := EncodeResponse(resp)
	if err != nil {
		log.Println("EncodeResponse错误：", err)
		c.werror(conn)
		return err
	}
	n, err = conn.Write(respBytes)
	if err != nil {
		log.Println("发送数据错误，错误：", err)
		return err
	}
	if n != len(respBytes) {
		log.Printf("发送数据长度错误，已经发送：%dbyte，总字节长：%dbyte\n", n, len(respBytes))
	} else {
		log.Printf("本次请求成功完成，共发送：%dbyte\n", n)
	}
	return nil
}
