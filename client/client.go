package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/kcp"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/lib/pool"
	"net"
	"sync"
	"time"
)

type TRPClient struct {
	svrAddr        string
	linkMap        map[int]*conn.Link
	stop           chan bool
	tunnel         *conn.Conn
	bridgeConnType string
	sync.Mutex
	vKey string
}

//new client
func NewRPClient(svraddr string, vKey string, bridgeConnType string) *TRPClient {
	return &TRPClient{
		svrAddr:        svraddr,
		linkMap:        make(map[int]*conn.Link),
		stop:           make(chan bool),
		Mutex:          sync.Mutex{},
		vKey:           vKey,
		bridgeConnType: bridgeConnType,
	}
}

//start
func (s *TRPClient) Start() error {
	s.NewConn()
	return nil
}

//新建
func (s *TRPClient) NewConn() {
	var err error
	var c net.Conn
retry:
	if s.bridgeConnType == "tcp" {
		c, err = net.Dial("tcp", s.svrAddr)
	} else {
		var sess *kcp.UDPSession
		sess, err = kcp.DialWithOptions(s.svrAddr, nil, 150, 3)
		conn.SetUdpSession(sess)
		c = sess
	}
	if err != nil {
		lg.Println("连接服务端失败,五秒后将重连")
		time.Sleep(time.Second * 5)
		goto retry
		return
	}
	s.processor(conn.NewConn(c))
}

//处理
func (s *TRPClient) processor(c *conn.Conn) {
	c.SetAlive(s.bridgeConnType)
	if _, err := c.Write([]byte(common.Getverifyval(s.vKey))); err != nil {
		return
	}
	c.WriteMain()
	go s.dealChan()
	for {
		flags, err := c.ReadFlag()
		if err != nil {
			lg.Println("服务端断开,正在重新连接")
			break
		}
		switch flags {
		case common.VERIFY_EER:
			lg.Fatalf("vKey:%s不正确,服务端拒绝连接,请检查", s.vKey)
		case common.NEW_CONN:
			if link, err := c.GetLinkInfo(); err != nil {
				break
			} else {
				s.Lock()
				s.linkMap[link.Id] = link
				s.Unlock()
				go s.linkProcess(link, c)
			}
		case common.RES_CLOSE:
			lg.Fatalln("该vkey被另一客户连接")
		case common.RES_MSG:
			lg.Println("服务端返回错误，重新连接")
			break
		default:
			lg.Println("无法解析该错误，重新连接")
			break
		}
	}
	s.stop <- true
	s.linkMap = make(map[int]*conn.Link)
	go s.NewConn()
}
func (s *TRPClient) linkProcess(link *conn.Link, c *conn.Conn) {
	//与目标建立连接
	server, err := net.DialTimeout(link.ConnType, link.Host, time.Second*3)

	if err != nil {
		c.WriteFail(link.Id)
		lg.Println("connect to ", link.Host, "error:", err)
		return
	}

	c.WriteSuccess(link.Id)

	link.Conn = conn.NewConn(server)
	buf := pool.BufPoolCopy.Get().([]byte)
	for {
		if n, err := server.Read(buf); err != nil {
			s.tunnel.SendMsg([]byte(common.IO_EOF), link)
			break
		} else {
			if _, err := s.tunnel.SendMsg(buf[:n], link); err != nil {
				c.Close()
				break
			}
		}
	}
	pool.PutBufPoolCopy(buf)
	s.Lock()
	delete(s.linkMap, link.Id)
	s.Unlock()
}

//隧道模式处理
func (s *TRPClient) dealChan() {
	var err error
	var c net.Conn
	var sess *kcp.UDPSession
	if s.bridgeConnType == "tcp" {
		c, err = net.Dial("tcp", s.svrAddr)
	} else {
		sess, err = kcp.DialWithOptions(s.svrAddr, nil, 10, 3)
		conn.SetUdpSession(sess)
		c = sess
	}
	if err != nil {
		lg.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	//验证
	if _, err := c.Write([]byte(common.Getverifyval(s.vKey))); err != nil {
		lg.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	//默认长连接保持
	s.tunnel = conn.NewConn(c)
	s.tunnel.SetAlive(s.bridgeConnType)
	//写标志
	s.tunnel.WriteChan()

	go func() {
		for {
			if id, err := s.tunnel.GetLen(); err != nil {
				lg.Println("get msg id error")
				break
			} else {
				s.Lock()
				if v, ok := s.linkMap[id]; ok {
					s.Unlock()
					if content, err := s.tunnel.GetMsgContent(v); err != nil {
						lg.Println("get msg content error:", err, id)
						pool.PutBufPoolCopy(content)
						break
					} else {
						if len(content) == len(common.IO_EOF) && string(content) == common.IO_EOF {
							v.Conn.Close()
						} else if v.Conn != nil {
							v.Conn.Write(content)
						}
						pool.PutBufPoolCopy(content)
					}
				} else {
					s.Unlock()
				}
			}
		}
	}()
	select {
	case <-s.stop:
		break
	}
}
