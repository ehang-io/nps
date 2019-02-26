package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"net"
	"os"
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
		logs.Error("The connection server failed and will be reconnected in five seconds")
		time.Sleep(time.Second * 5)
		goto retry
	}
	logs.Info("Successful connection with server %s", s.svrAddr)
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
			logs.Error("Accept server data error %s, end this service", err.Error())
			break
		}
		switch flags {
		case common.VERIFY_EER:
			logs.Error("VKey:%s is incorrect, the server refuses to connect, please check", s.vKey)
			os.Exit(0)
		case common.NEW_CONN:
			if link, err := c.GetLinkInfo(); err != nil {
				break
			} else {
				s.Lock()
				s.linkMap[link.Id] = link
				s.Unlock()
				link.MsgConn = s.msgTunnel
				go linkProcess(link, c, s.tunnel)
				link.RunWrite()
			}
		case common.RES_CLOSE:
			logs.Error("The authentication key is connected by another client or the server closes the client.")
			os.Exit(0)
		case common.RES_MSG:
			logs.Error("Server-side return error")
			break
		case common.NEW_UDP_CONN:
			//读取服务端地址、密钥 继续做处理
			if lAddr, err := c.GetLenContent(); err != nil {
				return
			} else if pwd, err := c.GetLenContent(); err == nil {
				logs.Warn(string(lAddr), string(pwd))
				go s.newUdpConn(string(lAddr), string(pwd))
			}
		default:
			logs.Warn("The error could not be resolved")
			break
		}
	}
	c.Close()
	s.Close()
}

func (s *TRPClient) newUdpConn(rAddr string, md5Password string) {
	tmpConn, err := net.Dial("udp", "114.114.114.114:53")
	if err != nil {
		logs.Warn(err)
		return
	}
	tmpConn.Close()
	//与服务端建立udp连接
	localAddr, _ := net.ResolveUDPAddr("udp", tmpConn.LocalAddr().String())
	localConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		logs.Warn(err)
		return
	}
	localKcpConn, err := kcp.NewConn(rAddr, nil, 150, 3, localConn)
	logs.Warn(localConn.RemoteAddr(), rAddr)
	conn.SetUdpSession(localKcpConn)
	if err != nil {
		logs.Warn(err)
		return
	}
	localToolConn := conn.NewConn(localKcpConn)
	//写入密钥、provider身份
	if _, err := localToolConn.Write([]byte(md5Password)); err != nil {
		logs.Warn(err)
		return
	}
	if _, err := localToolConn.Write([]byte(common.WORK_P2P_PROVIDER)); err != nil {
		logs.Warn(err)
		return
	}
	//接收服务端传的visitor地址
	if b, err := localToolConn.GetLenContent(); err != nil {
		logs.Warn(err)
		return
	} else {
		logs.Warn("收到服务端回传地址", string(b))
		//向visitor地址发送测试消息
		visitorAddr, err := net.ResolveUDPAddr("udp", string(b))
		if err != nil {
			logs.Warn(err)
		}
		logs.Warn(visitorAddr.String())
		if n, err := localConn.WriteTo([]byte("test"), visitorAddr); err != nil {
			logs.Warn(err)
		} else {
			logs.Warn("write", n)
		}
		//给服务端发反馈
		if _, err := localToolConn.Write([]byte(common.VERIFY_SUCCESS)); err != nil {
			logs.Warn(err)
		}
		//关闭与服务端的连接
		localConn.Close()
		//关闭与服务端udp conn，建立新的监听
		localConn, err = net.ListenUDP("udp", localAddr)

		if err != nil {
			logs.Warn(err)
		}
		l, err := kcp.ServeConn(nil, 150, 3, localConn)
		if err != nil {
			logs.Warn(err)
			return
		}
		for {
			//接收新的监听，得到conn，
			udpTunnel, err := l.AcceptKCP()
			logs.Warn(udpTunnel.RemoteAddr(), udpTunnel.LocalAddr())
			if err != nil {
				logs.Warn(err)
				l.Close()
				return
			}
			conn.SetUdpSession(udpTunnel)
			if udpTunnel.RemoteAddr().String() == string(b) {
				//读取link,设置msgCh 设置msgConn消息回传响应机制
				c, e := net.Dial("tcp", "123.206.77.88:22")
				if e != nil {
					logs.Warn(e)
					return
				}

				go common.CopyBuffer(c, udpTunnel)
				common.CopyBuffer(udpTunnel, c)
				//读取flag ping/new/msg/msgConn//分别对于不同的做法
				break
			}
		}

	}
}

func linkProcess(link *conn.Link, statusConn, msgConn *conn.Conn) {
	link.Host = common.FormatAddress(link.Host)
	//与目标建立连接
	server, err := net.DialTimeout(link.ConnType, link.Host, time.Second*3)

	if err != nil {
		statusConn.WriteFail(link.Id)
		logs.Warn("connect to ", link.Host, "error:", err)
		return
	}
	statusConn.WriteSuccess(link.Id)
	link.Conn = conn.NewConn(server)
	link.RunRead(msgConn)
}

func (s *TRPClient) getMsgStatus() {
	var err error
	s.msgTunnel, err = NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_SEND_STATUS, s.proxyUrl)
	if err != nil {
		logs.Error("connect to ", s.svrAddr, "error:", err)
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
		logs.Error("connect to ", s.svrAddr, "error:", err)
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

func (s *TRPClient) linkCleanSession() {
	ticker := time.NewTicker(time.Minute * 5)
	for {
		select {
		case <-ticker.C:
			s.Lock()
			for _, v := range s.linkMap {
				if v.FinishUse {
					delete(s.linkMap, v.Id)
				}
			}
			s.Unlock()
		}
	}
}
