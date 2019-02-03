package server

import (
	"errors"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/utils"
	"net"
	"net/http"
	"sync"
)

//server base struct
type server struct {
	id           int
	bridge       *bridge.Bridge
	task         *utils.Tunnel
	config       *utils.Config
	errorContent []byte
	sync.Mutex
}

func (s *server) FlowAdd(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.task.Flow.ExportFlow += out
	s.task.Flow.InletFlow += in
}

func (s *server) FlowAddHost(host *utils.Host, in, out int64) {
	s.Lock()
	defer s.Unlock()
	host.Flow.ExportFlow += out
	host.Flow.InletFlow += in
}

//热更新配置
func (s *server) ResetConfig() bool {
	//获取最新数据
	task, err := CsvDb.GetTask(s.task.Id)
	if err != nil {
		return false
	}
	if s.task.Client.Flow.FlowLimit > 0 && (s.task.Client.Flow.FlowLimit<<20) < (s.task.Client.Flow.ExportFlow+s.task.Client.Flow.InletFlow) {
		return false
	}
	s.task.UseClientCnf = task.UseClientCnf
	//使用客户端配置
	client, err := CsvDb.GetClient(s.task.Client.Id)
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
	s.config.CompressDecode, s.config.CompressEncode = utils.GetCompressType(s.config.Compress)
	return true
}

func (s *server) linkCopy(link *utils.Link, c *utils.Conn, rb []byte, tunnel *utils.Conn, flow *utils.Flow) {
	if rb != nil {
		if _, err := tunnel.SendMsg(rb, link); err != nil {
			c.Close()
			return
		}
		flow.Add(len(rb), 0)
	}
	for {
		buf := utils.BufPoolCopy.Get().([]byte)
		if n, err := c.Read(buf); err != nil {
			tunnel.SendMsg([]byte(utils.IO_EOF), link)
			break
		} else {
			if _, err := tunnel.SendMsg(buf[:n], link); err != nil {
				utils.PutBufPoolCopy(buf)
				c.Close()
				break
			}
			utils.PutBufPoolCopy(buf)
			flow.Add(n, 0)
		}
	}
}

func (s *server) writeConnFail(c net.Conn) {
	c.Write([]byte(utils.ConnectionFailBytes))
	c.Write(s.errorContent)
}

//权限认证
func (s *server) auth(r *http.Request, c *utils.Conn, u, p string) error {
	if u != "" && p != "" && !utils.CheckAuth(r, u, p) {
		c.Write([]byte(utils.UnauthorizedBytes))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}
