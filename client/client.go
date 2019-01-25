package client

import (
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type TRPClient struct {
	svrAddr      string
	tcpNum       int
	tunnelNum    int64
	tunnel       chan bool
	serverStatus bool
	sync.Mutex
	vKey string
}

//new client
func NewRPClient(svraddr string, tcpNum int, vKey string) *TRPClient {
	c := new(TRPClient)
	c.svrAddr = svraddr
	c.tcpNum = tcpNum
	c.vKey = vKey
	c.tunnel = make(chan bool)
	return c
}

//start
func (s *TRPClient) Start() error {
	for i := 0; i < s.tcpNum; i++ {
		go s.NewConn()
	}
	for i := 0; i < 5; i++ {
		go s.dealChan()
	}
	go s.session()
	return nil
}

//新建
func (s *TRPClient) NewConn() error {
	s.Lock()
	s.serverStatus = false
	conn, err := net.Dial("tcp", s.svrAddr)
	if err != nil {
		log.Println("连接服务端失败,五秒后将重连")
		time.Sleep(time.Second * 5)
		s.Unlock()
		go s.NewConn()
		return err
	}
	s.Unlock()
	return s.processor(utils.NewConn(conn))
}

//处理
func (s *TRPClient) processor(c *utils.Conn) error {
	s.serverStatus = true
	c.SetAlive()
	if _, err := c.Write([]byte(utils.Getverifyval(s.vKey))); err != nil {
		return err
	}
	c.WriteMain()
	for {
		flags, err := c.ReadFlag()
		if err != nil {
			log.Println("服务端断开,五秒后将重连", err)
			go s.NewConn()
			break
		}
		switch flags {
		case utils.VERIFY_EER:
			log.Fatalln("vkey:", s.vKey, "不正确,服务端拒绝连接,请检查")
		case utils.WORK_CHAN: //隧道模式，每次开启10个，加快连接速度
		case utils.RES_MSG:
			log.Println("服务端返回错误。")
		default:
			log.Println("无法解析该错误。", flags)
		}
	}
	return nil
}

//隧道模式处理
func (s *TRPClient) dealChan() {
	var err error
	//创建一个tcp连接
	conn, err := net.Dial("tcp", s.svrAddr)
	if err != nil {
		log.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	//验证
	if _, err := conn.Write([]byte(utils.Getverifyval(s.vKey))); err != nil {
		log.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	//默认长连接保持
	c := utils.NewConn(conn)
	c.SetAlive()
	//写标志
	c.WriteChan()
re:
	atomic.AddInt64(&s.tunnelNum, 1)
	//获取连接的host type(tcp or udp)
	typeStr, host, en, de, crypt, mux, err := c.GetHostFromConn()
	s.tunnel <- true
	atomic.AddInt64(&s.tunnelNum, -1)
	if err != nil {
		c.Close()
		return
	}
	s.ConnectAndCopy(c, typeStr, host, en, de, crypt, mux)
	if mux {
		utils.FlushConn(conn)
		goto re
	} else {
		c.Close()
	}
}

func (s *TRPClient) session() {
	t := time.NewTicker(time.Millisecond * 1000)
	for {
		select {
		case <-s.tunnel:
		case <-t.C:
		}
		if s.serverStatus && s.tunnelNum < 5 {
			go s.dealChan()
		}
	}
}

func (s *TRPClient) ConnectAndCopy(c *utils.Conn, typeStr, host string, en, de int, crypt, mux bool) {
	//与目标建立连接,超时时间为3
	server, err := net.DialTimeout(typeStr, host, time.Second*3)
	if err != nil {
		log.Println("connect to ", host, "error:", err, mux)
		c.WriteFail()
		return
	}
	c.WriteSuccess()
	utils.ReplayWaitGroup(c.Conn, server, en, de, crypt, mux)
}
