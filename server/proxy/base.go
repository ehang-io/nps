package proxy

import (
	"errors"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
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
func (s *BaseServer) DealClient(c *conn.Conn, addr string, rb []byte, tp string) error {
	link := conn.NewLink(tp, addr, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, c.Conn.RemoteAddr().String())

	if target, err := s.bridge.SendLinkInfo(s.task.Client.Id, link, c.Conn.RemoteAddr().String(), s.task); err != nil {
		logs.Warn("task id %d get connection from client id %d  error %s", s.task.Id, s.task.Client.Id, err.Error())
		c.Close()
		return err
	} else {
		if rb != nil {
			target.Write(rb)
		}
		conn.CopyWaitGroup(target, c.Conn, link.Crypt, link.Compress, s.task.Client.Rate, s.task.Client.Flow, true)
	}

	s.task.Client.AddConn()
	return nil
}
