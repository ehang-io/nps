package server

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

//server base struct
type server struct {
	id           int
	bridge       *bridge.Bridge
	task         *file.Tunnel
	config       *file.Config
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

//热更新配置
func (s *server) ResetConfig() bool {
	//获取最新数据
	task, err := file.GetCsvDb().GetTask(s.task.Id)
	if err != nil {
		return false
	}
	if s.task.Client.Flow.FlowLimit > 0 && (s.task.Client.Flow.FlowLimit<<20) < (s.task.Client.Flow.ExportFlow+s.task.Client.Flow.InletFlow) {
		return false
	}
	s.task.UseClientCnf = task.UseClientCnf
	//使用客户端配置
	client, err := file.GetCsvDb().GetClient(s.task.Client.Id)
	if s.task.UseClientCnf {
		if err == nil {
			s.config.U = client.Cnf.U
			s.config.P = client.Cnf.P
			s.config.Compress = client.Cnf.Compress
			s.config.Crypt = client.Cnf.Crypt
		}
	} else {
		if err == nil {
			s.config.U = task.Config.U
			s.config.P = task.Config.P
			s.config.Compress = task.Config.Compress
			s.config.Crypt = task.Config.Crypt
		}
	}
	s.task.Client.Rate = client.Rate
	s.config.CompressDecode, s.config.CompressEncode = common.GetCompressType(s.config.Compress)
	return true
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
