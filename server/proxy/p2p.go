package proxy

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
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
	kcpListener, err := kcp.ListenWithOptions(":"+strconv.Itoa(s.p2pPort), nil, 150, 3)
	if err != nil {
		logs.Error(err)
		return err
	}
	for {
		c, err := kcpListener.AcceptKCP()
		conn.SetUdpSession(c)
		if err != nil {
			logs.Warn(err)
			continue
		}
		go s.p2pProcess(conn.NewConn(c))
	}
	return nil
}

func (s *P2PServer) p2pProcess(c *conn.Conn) {
	logs.Warn("new link", c.Conn.RemoteAddr())
	//获取密钥
	var (
		f   string
		b   []byte
		err error
		v   *p2p
		ok  bool
	)
	if b, err = c.ReadLen(32); err != nil {
		return
	}
	//获取角色
	if f, err = c.ReadFlag(); err != nil {
		return
	}
	logs.Warn("收到", string(b), f)
	if v, ok = s.p2p[string(b)]; !ok {
		v = new(p2p)
		s.p2p[string(b)] = v
	}
	logs.Warn(f, c.Conn.RemoteAddr().String())
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
		logs.Warn("等待确认")
		if _, err := v.provider.ReadFlag(); err == nil {
			v.visitor.WriteLenContent([]byte(v.providerAddr))
			logs.Warn("收到确认")
			delete(s.p2p, string(b))
		} else {
			logs.Warn("收到确认失败", err)
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
	//假设是连接者、等待对应的被连接者连上后，发送被连接者信息
	//假设是被连接者，等待对应的连接者脸上后，发送连接者信息
}
