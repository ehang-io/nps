package server

import (
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"sync"
)

//server base struct
type server struct {
	bridge *bridge.Tunnel
	config *utils.ServerConfig
	sync.Mutex
}

func (s *server) GetTunnelAndWriteHost(connType string, cnf *utils.ServerConfig, addr string) (*utils.Conn, error) {
	var err error
	link, err := s.bridge.GetTunnel(cnf.ClientId, cnf.CompressEncode, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
	if err != nil {
		return nil, err
	}
	if _, err = link.WriteHost(connType, addr); err != nil {
		link.Close()
		return nil, err
	}
	return link, nil
}

func (s *server) FlowAdd(in, out int64) {
	s.Lock()
	defer s.Unlock()
	if s.config.Flow == nil {
		s.config.Flow = new(utils.Flow)
	}
	s.config.Flow.ExportFlow += out
	s.config.Flow.InletFlow += in
}

func (s *server) FlowAddHost(host *utils.HostList, in, out int64) {
	s.Lock()
	defer s.Unlock()
	if s.config.Flow == nil {
		s.config.Flow = new(utils.Flow)
	}
	host.Flow.ExportFlow += out
	host.Flow.InletFlow += in
}

//热更新配置
func (s *server) ResetConfig() {
	task, err := CsvDb.GetTask(s.config.Id)
	if err != nil {
		return
	}
	s.config.UseClientCnf = task.UseClientCnf
	if s.config.UseClientCnf {
		client, err := CsvDb.GetClient(s.config.ClientId)
		if err == nil {
			s.config.U = client.Cnf.U
			s.config.P = client.Cnf.P
			s.config.Compress = client.Cnf.Compress
			s.config.Mux = client.Cnf.Mux
			s.config.Crypt = client.Cnf.Crypt
		}
		s.config.CompressDecode, s.config.CompressEncode = utils.GetCompressType(client.Cnf.Compress)
	} else {
		if err == nil {
			s.config.U = task.U
			s.config.P = task.P
			s.config.Compress = task.Compress
			s.config.Mux = task.Mux
			s.config.Crypt = task.Crypt
		}
		s.config.CompressDecode, s.config.CompressEncode = utils.GetCompressType(task.Compress)
	}
}
