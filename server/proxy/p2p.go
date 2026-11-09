package proxy

import (
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"github.com/astaxie/beego/logs"
)

type P2PServer struct {
	BaseServer
	p2pPort  int
	p2p      map[string]*p2p
	mu       sync.Mutex // guards p2p and the addrs inside it
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
	s.listener, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: s.p2pPort})
	if err != nil {
		return err
	}
	for {
		buf := common.BufPoolUdp.Get().([]byte)
		n, addr, err := s.listener.ReadFromUDP(buf)
		if err != nil {
			common.PutBufPoolUdp(buf)
			if errors.Is(err, net.ErrClosed) {
				break
			}
			continue
		}
		// the payload is copied out, the pooled buffer goes straight back
		str := string(buf[:n])
		common.PutBufPoolUdp(buf)
		go s.handleP2P(addr, str)
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
	s.mu.Lock()
	if v, ok = s.p2p[arr[0]]; !ok {
		v = new(p2p)
		s.p2p[arr[0]] = v
	}
	if arr[1] == common.WORK_P2P_VISITOR {
		v.visitorAddr = addr
	} else {
		v.providerAddr = addr
	}
	s.mu.Unlock()
	logs.Trace("new p2p connection ,role %s , password %s ,local address %s", arr[1], arr[0], addr.String())
	if arr[1] == common.WORK_P2P_VISITOR {
		for i := 20; i > 0; i-- {
			s.mu.Lock()
			provider := v.providerAddr
			visitor := v.visitorAddr
			s.mu.Unlock()
			if provider != nil && visitor != nil {
				s.listener.WriteTo([]byte(provider.String()), visitor)
				s.listener.WriteTo([]byte(visitor.String()), provider)
				break
			}
			time.Sleep(time.Second)
		}
		s.mu.Lock()
		delete(s.p2p, arr[0])
		s.mu.Unlock()
	}
}
