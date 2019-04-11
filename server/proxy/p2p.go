package proxy

import (
	"encoding/binary"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"net"
	"strconv"
	"time"
)

type P2PServer struct {
	BaseServer
	p2pPort int
	p2p     map[string]*p2p
}

type p2p struct {
	provider        *conn.Conn
	visitor         *conn.Conn
	visitorAddr     string
	providerAddr    string
	providerNatType int32
	visitorNatType  int32
}

func NewP2PServer(p2pPort int) *P2PServer {
	return &P2PServer{
		p2pPort: p2pPort,
		p2p:     make(map[string]*p2p),
	}
}

func (s *P2PServer) Start() error {
	return conn.NewKcpListenerAndProcess(":"+strconv.Itoa(s.p2pPort), func(c net.Conn) {
		s.p2pProcess(conn.NewConn(c))
	})
}

func (s *P2PServer) p2pProcess(c *conn.Conn) {
	var (
		f       string
		b       []byte
		err     error
		v       *p2p
		ok      bool
		natType int32
	)
	if b, err = c.GetShortContent(32); err != nil {
		return
	}
	//get role
	if f, err = c.ReadFlag(); err != nil {
		return
	}
	//get nat type
	if err := binary.Read(c, binary.LittleEndian, &natType); err != nil {
		return
	}
	if v, ok = s.p2p[string(b)]; !ok {
		v = new(p2p)
		s.p2p[string(b)] = v
	}
	logs.Trace("new p2p connection ,role %s , password %s, nat type %s ,local address %s", f, string(b), strconv.Itoa(int(natType)), c.RemoteAddr().String())
	//存储
	if f == common.WORK_P2P_VISITOR {
		v.visitorAddr = c.Conn.RemoteAddr().String()
		v.visitorNatType = natType
		v.visitor = c
		for i := 20; i > 0; i-- {
			if v.provider != nil {
				v.provider.WriteLenContent([]byte(v.visitorAddr))
				binary.Write(v.provider, binary.LittleEndian, v.visitorNatType)
				break
			}
			time.Sleep(time.Second)
		}
		v.provider = nil
	} else {
		v.providerAddr = c.Conn.RemoteAddr().String()
		v.providerNatType = natType
		v.provider = c
		for i := 20; i > 0; i-- {
			if v.visitor != nil {
				v.visitor.WriteLenContent([]byte(v.providerAddr))
				binary.Write(v.visitor, binary.LittleEndian, v.providerNatType)
				break
			}
			time.Sleep(time.Second)
		}
		v.visitor = nil
	}
}
