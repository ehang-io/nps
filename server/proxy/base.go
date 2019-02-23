package proxy

import (
	"errors"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/pool"
	"net"
	"net/http"
	"sync"
)

type Service interface {
	Start() error
	Close() error
}

//Server BaseServer struct
type BaseServer struct {
	id           int
	bridge       *bridge.Bridge
	task         *file.Tunnel
	errorContent []byte
	sync.Mutex
}

func NewBaseServer(bridge *bridge.Bridge, task *file.Tunnel) *BaseServer {
	return &BaseServer{
		bridge:       bridge,
		task:         task,
		errorContent: nil,
		Mutex:        sync.Mutex{},
	}
}

func (s *BaseServer) FlowAdd(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.task.Flow.ExportFlow += out
	s.task.Flow.InletFlow += in
}

func (s *BaseServer) FlowAddHost(host *file.Host, in, out int64) {
	s.Lock()
	defer s.Unlock()
	host.Flow.ExportFlow += out
	host.Flow.InletFlow += in
}

func (s *BaseServer) linkCopy(link *conn.Link, c *conn.Conn, rb []byte, tunnel *conn.Conn, flow *file.Flow) {
	if rb != nil {
		if _, err := tunnel.SendMsg(rb, link); err != nil {
			c.Close()
			return
		}
		flow.Add(len(rb), 0)
		<-link.StatusCh
	}

	buf := pool.BufPoolCopy.Get().([]byte)
	for {
		if err := s.checkFlow(); err != nil {
			c.Close()
			break
		}
		if n, err := c.Read(buf); err != nil {
			tunnel.SendMsg([]byte(common.IO_EOF), link)
			break
		} else {
			if _, err := tunnel.SendMsg(buf[:n], link); err != nil {
				c.Close()
				break
			}
			flow.Add(n, 0)
		}
		<-link.StatusCh
	}
	s.task.Client.AddConn()
	pool.PutBufPoolCopy(buf)
}

func (s *BaseServer) writeConnFail(c net.Conn) {
	c.Write([]byte(common.ConnectionFailBytes))
	c.Write(s.errorContent)
}

//权限认证
func (s *BaseServer) auth(r *http.Request, c *conn.Conn, u, p string) error {
	if u != "" && p != "" && !common.CheckAuth(r, u, p) {
		c.Write([]byte(common.UnauthorizedBytes))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}

func (s *BaseServer) checkFlow() error {
	if s.task.Client.Flow.FlowLimit > 0 && (s.task.Client.Flow.FlowLimit<<20) < (s.task.Client.Flow.ExportFlow+s.task.Client.Flow.InletFlow) {
		return errors.New("Traffic exceeded")
	}
	return nil
}

//与客户端建立通道
func (s *BaseServer) DealClient(c *conn.Conn, addr string, rb []byte) error {
	link := conn.NewLink(s.task.Client.GetId(), common.CONN_TCP, addr, s.task.Client.Cnf.CompressEncode, s.task.Client.Cnf.CompressDecode, s.task.Client.Cnf.Crypt, c, s.task.Flow, nil, s.task.Client.Rate, nil)

	if tunnel, err := s.bridge.SendLinkInfo(s.task.Client.Id, link, c.Conn.RemoteAddr().String()); err != nil {
		c.Close()
		return err
	} else {
		link.Run(true)
		s.linkCopy(link, c, rb, tunnel, s.task.Flow)
	}
	return nil
}
