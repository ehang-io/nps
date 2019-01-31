package client

import (
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"sync"
	"time"
)

type TRPClient struct {
	svrAddr string
	linkMap map[int]*utils.Link
	stop    chan bool
	tunnel  *utils.Conn
	sync.Mutex
	vKey string
}

//new client
func NewRPClient(svraddr string, vKey string) *TRPClient {
	return &TRPClient{
		svrAddr: svraddr,
		linkMap: make(map[int]*utils.Link),
		stop:    make(chan bool),
		tunnel:  nil,
		Mutex:   sync.Mutex{},
		vKey:    vKey,
	}
}

//start
func (s *TRPClient) Start() error {
	s.NewConn()
	return nil
}

//新建
func (s *TRPClient) NewConn() {
retry:
	conn, err := net.Dial("tcp", s.svrAddr)
	if err != nil {
		log.Println("连接服务端失败,五秒后将重连")
		time.Sleep(time.Second * 5)
		goto retry
		return
	}
	s.processor(utils.NewConn(conn))
}

//处理
func (s *TRPClient) processor(c *utils.Conn) {
	c.SetAlive()
	if _, err := c.Write([]byte(utils.Getverifyval(s.vKey))); err != nil {
		return
	}
	c.WriteMain()

	go s.dealChan()

	for {
		flags, err := c.ReadFlag()
		if err != nil {
			log.Println("服务端断开,正在重新连接")
			break
		}
		switch flags {
		case utils.VERIFY_EER:
			log.Fatalf("vKey:%s不正确,服务端拒绝连接,请检查", s.vKey)
		case utils.NEW_CONN:
			if link, err := c.GetLinkInfo(); err != nil {
				break
			} else {
				log.Println(link)
				s.Lock()
				s.linkMap[link.Id] = link
				s.Unlock()
				go s.linkProcess(link, c)
			}
		case utils.RES_CLOSE:
			log.Fatal("该vkey被另一客户连接")
		case utils.RES_MSG:
			log.Println("服务端返回错误，重新连接")
			break
		default:
			log.Println("无法解析该错误，重新连接")
			break
		}
	}
	s.stop <- true
	s.linkMap = make(map[int]*utils.Link)
	go s.NewConn()
}
func (s *TRPClient) linkProcess(link *utils.Link, c *utils.Conn) {
	//与目标建立连接
	server, err := net.DialTimeout(link.ConnType, link.Host, time.Second*3)

	if err != nil {
		c.WriteFail(link.Id)
		log.Println("connect to ", link.Host, "error:", err)
		return
	}

	c.WriteSuccess(link.Id)

	link.Conn = utils.NewConn(server)

	for {
		buf := utils.BufPoolCopy.Get().([]byte)
		if n, err := server.Read(buf); err != nil {
			utils.PutBufPoolCopy(buf)
			s.tunnel.SendMsg([]byte(utils.IO_EOF), link)
			break
		} else {
			if _, err := s.tunnel.SendMsg(buf[:n], link); err != nil {
				utils.PutBufPoolCopy(buf)
				c.Close()
				break
			}
			utils.PutBufPoolCopy(buf)
			if link.ConnType == utils.CONN_UDP {
				c.Close()
				break
			}
		}
	}

	s.Lock()
	delete(s.linkMap, link.Id)
	s.Unlock()
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
	s.tunnel = utils.NewConn(conn)
	s.tunnel.SetAlive()
	//写标志
	s.tunnel.WriteChan()

	go func() {
		for {
			if id, err := s.tunnel.GetLen(); err != nil {
				log.Println("get msg id error")
				break
			} else {
				s.Lock()
				if v, ok := s.linkMap[id]; ok {
					s.Unlock()
					if content, err := s.tunnel.GetMsgContent(v); err != nil {
						log.Println("get msg content error:", err, id)
						break
					} else {
						if len(content) == len(utils.IO_EOF) && string(content) == utils.IO_EOF {
							v.Conn.Close()
						} else if v.Conn != nil {
							v.Conn.Write(content)
						}
					}
				} else {
					s.Unlock()
				}
			}
		}
	}()
	select {
	case <-s.stop:
	}
}
