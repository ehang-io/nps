package proxy

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
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
	provider     *conn.Conn
	visitor      *conn.Conn
	visitorAddr  string
	providerAddr string
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
	//获取密钥
	var (
		f   string
		b   []byte
		err error
		v   *p2p
		ok  bool
	)
	if b, err = c.GetShortContent(32); err != nil {
		return
	}
	//获取角色
	if f, err = c.ReadFlag(); err != nil {
		return
	}
	if v, ok = s.p2p[string(b)]; !ok {
		v = new(p2p)
		s.p2p[string(b)] = v
	}
	//存储
	if f == common.WORK_P2P_VISITOR {
		v.visitorAddr = c.Conn.RemoteAddr().String()
		v.visitor = c
		for {
			time.Sleep(time.Second)
			if v.provider != nil {
				break
			}
		}
		if _, err := v.provider.ReadFlag(); err == nil {
			v.visitor.WriteLenContent([]byte(v.providerAddr))
			delete(s.p2p, string(b))
		} else {
		}
	} else {
		v.providerAddr = c.Conn.RemoteAddr().String()
		v.provider = c
		for {
			time.Sleep(time.Second)
			if v.visitor != nil {
				v.provider.WriteLenContent([]byte(v.visitorAddr))
				break
			}
		}
	}
}
