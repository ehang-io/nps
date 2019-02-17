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

//server base struct
type server struct {
	id           int
	bridge       *bridge.Bridge
	task         *file.Tunnel
	errorContent []byte
	sync.Mutex
}

func (s *server) FlowAdd(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.task.Flow.ExportFlow += out
	s.task.Flow.InletFlow += in
}

func (s *server) FlowAddHost(host *file.Host, in, out int64) {
	s.Lock()
	defer s.Unlock()
	host.Flow.ExportFlow += out
	host.Flow.InletFlow += in
}

func (s *server) linkCopy(link *conn.Link, c *conn.Conn, rb []byte, tunnel *conn.Conn, flow *file.Flow) {
	if rb != nil {
		if _, err := tunnel.SendMsg(rb, link); err != nil {
			c.Close()
			return
		}
		flow.Add(len(rb), 0)
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
	pool.PutBufPoolCopy(buf)
}

func (s *server) writeConnFail(c net.Conn) {
	c.Write([]byte(common.ConnectionFailBytes))
	c.Write(s.errorContent)
}

//权限认证
func (s *server) auth(r *http.Request, c *conn.Conn, u, p string) error {
	if u != "" && p != "" && !common.CheckAuth(r, u, p) {
		c.Write([]byte(common.UnauthorizedBytes))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}

func (s *server) checkFlow() error {
	if s.task.Client.Flow.FlowLimit > 0 && (s.task.Client.Flow.FlowLimit<<20) < (s.task.Client.Flow.ExportFlow+s.task.Client.Flow.InletFlow) {
		return errors.New("Traffic exceeded")
	}
	return nil
}
