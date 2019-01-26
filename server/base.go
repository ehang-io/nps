package server

import (
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"sync"
)

//server base struct
type server struct {
	bridge *bridge.Bridge
	task   *utils.Tunnel
	config *utils.Config
	sync.Mutex
}

func (s *server) GetTunnelAndWriteHost(connType string, clientId int, cnf *utils.Config, addr string) (link *utils.Conn, err error) {
	if link, err = s.bridge.GetTunnel(clientId, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux); err != nil {
		return
	}
	if _, err = link.WriteHost(connType, addr); err != nil {
		link.Close()
	}
	return
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
func (s *server) ResetConfig() {
	//获取最新数据
	task, err := CsvDb.GetTask(s.task.Id)
	if err != nil {
		return
	}
	s.task.UseClientCnf = task.UseClientCnf
	//使用客户端配置
	if s.task.UseClientCnf {
		client, err := CsvDb.GetClient(s.task.Client.Id)
		if err == nil {
			s.config.U = client.Cnf.U
			s.config.P = client.Cnf.P
			s.config.Compress = client.Cnf.Compress
			s.config.Mux = client.Cnf.Mux
			s.config.Crypt = client.Cnf.Crypt
		}
	} else {
		if err == nil {
			s.config.U = task.Config.U
			s.config.P = task.Config.P
			s.config.Compress = task.Config.Compress
			s.config.Mux = task.Config.Mux
			s.config.Crypt = task.Config.Crypt
		}
	}
	s.config.CompressDecode, s.config.CompressEncode = utils.GetCompressType(s.config.Compress)
}
