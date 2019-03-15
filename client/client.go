package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/mux"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"net"
	"os"
	"time"
)

type TRPClient struct {
	svrAddr        string
	bridgeConnType string
	stop           chan bool
	proxyUrl       string
	vKey           string
	tunnel         *mux.Mux
	signal         *conn.Conn
	cnf            *config.Config
}

//new client
func NewRPClient(svraddr string, vKey string, bridgeConnType string, proxyUrl string, cnf *config.Config) *TRPClient {
	return &TRPClient{
		svrAddr:        svraddr,
		vKey:           vKey,
		bridgeConnType: bridgeConnType,
		stop:           make(chan bool),
		proxyUrl:       proxyUrl,
		cnf:            cnf,
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
	go s.ping()
	s.processor(c)
}

func (s *TRPClient) Close() {
	s.stop <- true
	s.signal.Close()
}

//处理
func (s *TRPClient) processor(c *conn.Conn) {
	s.signal = c
	go s.dealChan()
	if s.cnf != nil && len(s.cnf.Healths) > 0 {
		go heathCheck(s.cnf.Healths, s.signal)
	}
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
		case common.RES_CLOSE:
			logs.Error("The authentication key is connected by another client or the server closes the client.")
			os.Exit(0)
		case common.RES_MSG:
			logs.Error("Server-side return error")
			break
		case common.NEW_UDP_CONN:
			//读取服务端地址、密钥 继续做处理
			if lAddr, err := c.GetShortLenContent(); err != nil {
				logs.Warn(err)
				return
			} else if pwd, err := c.GetShortLenContent(); err == nil {
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
	tmpConn, err := common.GetLocalUdpAddr()
	if err != nil {
		logs.Error(err)
		return
	}
	localAddr, _ := net.ResolveUDPAddr("udp", tmpConn.LocalAddr().String())
	localConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		logs.Error(err)
		return
	}
	localKcpConn, err := kcp.NewConn(rAddr, nil, 150, 3, localConn)
	if err != nil {
		logs.Error(err)
		return
	}
	conn.SetUdpSession(localKcpConn)
	localToolConn := conn.NewConn(localKcpConn)
	//写入密钥、provider身份
	if _, err := localToolConn.Write([]byte(md5Password)); err != nil {
		logs.Error(err)
		return
	}
	if _, err := localToolConn.Write([]byte(common.WORK_P2P_PROVIDER)); err != nil {
		logs.Error(err)
		return
	}
	//接收服务端传的visitor地址
	var b []byte
	if b, err = localToolConn.GetShortLenContent(); err != nil {
		logs.Error(err)
		return
	}
	//向visitor地址发送测试消息
	visitorAddr, err := net.ResolveUDPAddr("udp", string(b))
	if err != nil {
		logs.Error(err)
		return
	}
	//向目标IP发送探测包
	if _, err := localConn.WriteTo([]byte("test"), visitorAddr); err != nil {
		logs.Error(err)
		return
	}
	//给服务端发反馈
	if _, err := localToolConn.Write([]byte(common.VERIFY_SUCCESS)); err != nil {
		logs.Error(err)
		return
	}
	//关闭与服务端的连接
	localConn.Close()
	//关闭与服务端udp conn，建立新的监听
	if localConn, err = net.ListenUDP("udp", localAddr); err != nil {
		logs.Error(err)
		return
	}
	l, err := kcp.ServeConn(nil, 150, 3, localConn)
	if err != nil {
		logs.Error(err)
		return
	}
	//接收新的监听，得到conn，
	for {
		udpTunnel, err := l.AcceptKCP()
		if err != nil {
			logs.Error(err)
			l.Close()
			return
		}
		if udpTunnel.RemoteAddr().String() == string(b) {
			conn.SetUdpSession(udpTunnel)
			//读取link,设置msgCh 设置msgConn消息回传响应机制
			l := mux.NewMux(udpTunnel)
			for {
				connMux, err := l.Accept()
				if err != nil {
					continue
				}
				go s.srcProcess(connMux)
			}
		}
	}
}

//mux tunnel
func (s *TRPClient) dealChan() {
	tunnel, err := NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_CHAN, s.proxyUrl)
	if err != nil {
		logs.Error("connect to ", s.svrAddr, "error:", err)
		return
	}
	go func() {
		s.tunnel = mux.NewMux(tunnel.Conn)
		for {
			src, err := s.tunnel.Accept()
			if err != nil {
				logs.Warn(err)
				break
			}
			go s.srcProcess(src)
		}
	}()
	<-s.stop
}

func (s *TRPClient) srcProcess(src net.Conn) {
	lk, err := conn.NewConn(src).GetLinkInfo()
	if err != nil {
		src.Close()
		logs.Error("get connection info from server error ", err)
		return
	}
	//host for target processing
	lk.Host = common.FormatAddress(lk.Host)
	//connect to target
	if targetConn, err := net.Dial(lk.ConnType, lk.Host); err != nil {
		logs.Warn("connect to %s error %s", lk.Host, err.Error())
		src.Close()
	} else {
		logs.Trace("new %s connection with the goal of %s, remote address:%s", lk.ConnType, lk.Host, lk.RemoteAddr)
		conn.CopyWaitGroup(src, targetConn, lk.Crypt, lk.Compress, nil, nil, false)
	}
}

func (s *TRPClient) ping() {
	ticker := time.NewTicker(time.Second * 5)
loop:
	for {
		select {
		case <-ticker.C:
			if s.tunnel.IsClose {
				s.Close()
				ticker.Stop()
				break loop
			}
		}
	}
}
