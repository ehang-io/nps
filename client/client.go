package client

import (
	"github.com/cnlh/nps/lib"
	"net"
	"sync"
	"time"
)

type TRPClient struct {
	svrAddr string
	linkMap map[int]*lib.Link
	stop    chan bool
	tunnel  *lib.Conn
	sync.Mutex
	vKey string
}

//new client
func NewRPClient(svraddr string, vKey string) *TRPClient {
	return &TRPClient{
		svrAddr: svraddr,
		linkMap: make(map[int]*lib.Link),
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
		lib.Println("连接服务端失败,五秒后将重连")
		time.Sleep(time.Second * 5)
		goto retry
		return
	}
	s.processor(lib.NewConn(conn))
}

//处理
func (s *TRPClient) processor(c *lib.Conn) {
	c.SetAlive()
	if _, err := c.Write([]byte(lib.Getverifyval(s.vKey))); err != nil {
		return
	}
	c.WriteMain()

	go s.dealChan()

	for {
		flags, err := c.ReadFlag()
		if err != nil {
			lib.Println("服务端断开,正在重新连接")
			break
		}
		switch flags {
		case lib.VERIFY_EER:
			lib.Fatalf("vKey:%s不正确,服务端拒绝连接,请检查", s.vKey)
		case lib.NEW_CONN:
			if link, err := c.GetLinkInfo(); err != nil {
				break
			} else {
				s.Lock()
				s.linkMap[link.Id] = link
				s.Unlock()
				go s.linkProcess(link, c)
			}
		case lib.RES_CLOSE:
			lib.Fatalln("该vkey被另一客户连接")
		case lib.RES_MSG:
			lib.Println("服务端返回错误，重新连接")
			break
		default:
			lib.Println("无法解析该错误，重新连接")
			break
		}
	}
	s.stop <- true
	s.linkMap = make(map[int]*lib.Link)
	go s.NewConn()
}
func (s *TRPClient) linkProcess(link *lib.Link, c *lib.Conn) {
	//与目标建立连接
	server, err := net.DialTimeout(link.ConnType, link.Host, time.Second*3)

	if err != nil {
		c.WriteFail(link.Id)
		lib.Println("connect to ", link.Host, "error:", err)
		return
	}

	c.WriteSuccess(link.Id)

	link.Conn = lib.NewConn(server)

	for {
		buf := lib.BufPoolCopy.Get().([]byte)
		if n, err := server.Read(buf); err != nil {
			lib.PutBufPoolCopy(buf)
			s.tunnel.SendMsg([]byte(lib.IO_EOF), link)
			break
		} else {
			if _, err := s.tunnel.SendMsg(buf[:n], link); err != nil {
				lib.PutBufPoolCopy(buf)
				c.Close()
				break
			}
			lib.PutBufPoolCopy(buf)
			//if link.ConnType == utils.CONN_UDP {
			//	c.Close()
			//	break
			//}
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
		lib.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	//验证
	if _, err := conn.Write([]byte(lib.Getverifyval(s.vKey))); err != nil {
		lib.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	//默认长连接保持
	s.tunnel = lib.NewConn(conn)
	s.tunnel.SetAlive()
	//写标志
	s.tunnel.WriteChan()

	go func() {
		for {
			if id, err := s.tunnel.GetLen(); err != nil {
				lib.Println("get msg id error")
				break
			} else {
				s.Lock()
				if v, ok := s.linkMap[id]; ok {
					s.Unlock()
					if content, err := s.tunnel.GetMsgContent(v); err != nil {
						lib.Println("get msg content error:", err, id)
						break
					} else {
						if len(content) == len(lib.IO_EOF) && string(content) == lib.IO_EOF {
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
