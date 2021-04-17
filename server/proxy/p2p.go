package proxy

import (
	"net"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"github.com/astaxie/beego/logs"
)

type P2PServer struct {
	BaseServer
	p2pPort  int
	p2p      map[string]*p2p
	listener *net.UDPConn
}

type p2p struct {
	visitorAddr  *net.UDPAddr
	providerAddr *net.UDPAddr
}

func NewP2PServer(p2pPort int) *P2PServer {
	return &P2PServer{
		p2pPort: p2pPort,
		p2p:     make(map[string]*p2p),
	}
}

func (s *P2PServer) Start() error {
	logs.Info("start p2p server port", s.p2pPort)
	var err error
	s.listener, err = net.ListenUDP("udp", &net.UDPAddr{net.ParseIP("0.0.0.0"), s.p2pPort, ""})
	if err != nil {
		return err
	}
	for {
		buf := common.BufPoolUdp.Get().([]byte)
		n, addr, err := s.listener.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			continue
		}
		go s.handleP2P(addr, string(buf[:n]))
	}
	return nil
}

func (s *P2PServer) handleP2P(addr *net.UDPAddr, str string) {
	var (
		v  *p2p
		ok bool
	)
	arr := strings.Split(str, common.CONN_DATA_SEQ)
	if len(arr) < 2 {
		return
	}
	if v, ok = s.p2p[arr[0]]; !ok {
		v = new(p2p)
		s.p2p[arr[0]] = v
	}
	logs.Trace("new p2p connection ,role %s , password %s ,local address %s", arr[1], arr[0], addr.String())
	if arr[1] == common.WORK_P2P_VISITOR {
		v.visitorAddr = addr
		for i := 20; i > 0; i-- {
			if v.providerAddr != nil {
				s.listener.WriteTo([]byte(v.providerAddr.String()), v.visitorAddr)
				s.listener.WriteTo([]byte(v.visitorAddr.String()), v.providerAddr)
				break
			}
			time.Sleep(time.Second)
		}
		delete(s.p2p, arr[0])
	} else {
		v.providerAddr = addr
	}
}
