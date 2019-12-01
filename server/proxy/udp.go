package proxy

import (
	"net"
	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
)

type UdpModeServer struct {
	BaseServer
	listener *net.UDPConn
}

func NewUdpModeServer(bridge *bridge.Bridge, task *file.Tunnel) *UdpModeServer {
	s := new(UdpModeServer)
	s.bridge = bridge
	s.task = task
	return s
}

//开始
func (s *UdpModeServer) Start() error {
	var err error
	if s.task.ServerIp == "" {
		s.task.ServerIp = "0.0.0.0"
	}
	s.listener, err = net.ListenUDP("udp", &net.UDPAddr{net.ParseIP(s.task.ServerIp), s.task.Port, ""})
	if err != nil {
		return err
	}
	buf := common.BufPoolUdp.Get().([]byte)
	for {
		n, addr, err := s.listener.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			continue
		}
		logs.Trace("New udp connection,client %d,remote address %s", s.task.Client.Id, addr)
		go s.process(addr, buf[:n])
	}
	return nil
}

func (s *UdpModeServer) process(addr *net.UDPAddr, data []byte) {
	if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
		logs.Warn("client id %d, task id %d,error %s, when udp connection", s.task.Client.Id, s.task.Id, err.Error())
		return
	}
	defer s.task.Client.AddConn()
	link := conn.NewLink(common.CONN_UDP, s.task.Target.TargetStr, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, addr.String(), s.task.Target.LocalProxy)
	if target, err := s.bridge.SendLinkInfo(s.task.Client.Id, link, s.task); err != nil {
		return
	} else {
		s.task.Flow.Add(int64(len(data)), 0)
		buf := common.BufPoolUdp.Get().([]byte)
		defer common.BufPoolUdp.Put(buf)
		target.Write(data)
		s.task.Flow.Add(int64(len(data)), 0)
		if n, err := target.Read(buf); err != nil {
			logs.Warn(err)
			return
		} else {
			s.listener.WriteTo(buf[:n], addr)
			s.task.Flow.Add(0, int64(n))
		}
	}
}

func (s *UdpModeServer) Close() error {
	return s.listener.Close()
}
