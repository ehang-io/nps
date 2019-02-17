package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/lib/pool"
	"net"
	"sync"
	"time"
)

type TRPClient struct {
	svrAddr        string
	linkMap        map[int]*conn.Link
	tunnel         *conn.Conn
	msgTunnel      *conn.Conn
	bridgeConnType string
	stop           chan bool
	proxyUrl       string
	sync.Mutex
	vKey string
}

//new client
func NewRPClient(svraddr string, vKey string, bridgeConnType string, proxyUrl string) *TRPClient {
	return &TRPClient{
		svrAddr:        svraddr,
		linkMap:        make(map[int]*conn.Link),
		Mutex:          sync.Mutex{},
		vKey:           vKey,
		bridgeConnType: bridgeConnType,
		stop:           make(chan bool),
		proxyUrl:       proxyUrl,
	}
}

//start
func (s *TRPClient) Start() {
retry:
	c, err := NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_MAIN, s.proxyUrl)
	if err != nil {
		lg.Println("The connection server failed and will be reconnected in five seconds")
		time.Sleep(time.Second * 5)
		goto retry
	}
	lg.Printf("Successful connection with server %s", s.svrAddr)
	s.processor(c)
}

func (s *TRPClient) Close() {
	s.tunnel.Close()
	s.stop <- true
	for _, v := range s.linkMap {
		if v.Conn != nil {
			v.Conn.Close()
		}
	}
}

//处理
func (s *TRPClient) processor(c *conn.Conn) {
	go s.dealChan()
	go s.getMsgStatus()
	for {
		flags, err := c.ReadFlag()
		if err != nil {
			lg.Printf("Accept server data error %s, end this service", err.Error())
			break
		}
		switch flags {
		case common.VERIFY_EER:
			lg.Fatalf("VKey:%s is incorrect, the server refuses to connect, please check", s.vKey)
		case common.NEW_CONN:
			if link, err := c.GetLinkInfo(); err != nil {
				break
			} else {
				s.Lock()
				s.linkMap[link.Id] = link
				s.Unlock()
				link.MsgConn = s.msgTunnel
				go s.linkProcess(link, c)
				link.Run(false)
			}
		case common.RES_CLOSE:
			lg.Fatalln("The authentication key is connected by another client or the server closes the client.")
		case common.RES_MSG:
			lg.Println("Server-side return error")
			break
		default:
			lg.Println("The error could not be resolved")
			break
		}
	}
	c.Close()
	s.Close()
}

func (s *TRPClient) linkProcess(link *conn.Link, c *conn.Conn) {
	link.Host = common.FormatAddress(link.Host)
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
			if link.ConnType == common.CONN_UDP {
				break
			}
		}
		<-link.StatusCh
	}
	pool.PutBufPoolCopy(buf)
	s.Lock()
	//TODO 删除map
	s.Unlock()
}

func (s *TRPClient) getMsgStatus() {
	var err error
	s.msgTunnel, err = NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_SEND_STATUS, s.proxyUrl)
	if err != nil {
		lg.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	go func() {
		for {
			if id, err := s.msgTunnel.GetLen(); err != nil {
				break
			} else {
				s.Lock()
				if v, ok := s.linkMap[id]; ok {
					s.Unlock()
					v.StatusCh <- true
				} else {
					s.Unlock()
				}
			}
		}
	}()
	<-s.stop
}

//隧道模式处理
func (s *TRPClient) dealChan() {
	var err error
	s.tunnel, err = NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_CHAN, s.proxyUrl)
	if err != nil {
		lg.Println("connect to ", s.svrAddr, "error:", err)
		return
	}
	go func() {
		for {
			if id, err := s.tunnel.GetLen(); err != nil {
				break
			} else {
				s.Lock()
				if v, ok := s.linkMap[id]; ok {
					s.Unlock()
					if content, err := s.tunnel.GetMsgContent(v); err != nil {
						pool.PutBufPoolCopy(content)
						break
					} else {
						v.MsgCh <- content
					}
				} else {
					s.Unlock()
				}
			}
		}
	}()
	<-s.stop
}
