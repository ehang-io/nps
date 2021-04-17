package proxy

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego/logs"
)

type UdpModeServer struct {
	BaseServer
	addrMap  sync.Map
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
	for {
		buf := common.BufPoolUdp.Get().([]byte)
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
	if v, ok := s.addrMap.Load(addr.String()); ok {
		clientConn, ok := v.(io.ReadWriteCloser)
		if ok {
			clientConn.Write(data)
			s.task.Flow.Add(int64(len(data)), 0)
		}
	} else {
		if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
			logs.Warn("client id %d, task id %d,error %s, when udp connection", s.task.Client.Id, s.task.Id, err.Error())
			return
		}
		defer s.task.Client.AddConn()
		link := conn.NewLink(common.CONN_UDP, s.task.Target.TargetStr, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, addr.String(), s.task.Target.LocalProxy)
		if clientConn, err := s.bridge.SendLinkInfo(s.task.Client.Id, link, s.task); err != nil {
			return
		} else {
			target := conn.GetConn(clientConn, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, nil, true)
			s.addrMap.Store(addr.String(), target)
			defer target.Close()

			target.Write(data)

			buf := common.BufPoolUdp.Get().([]byte)
			defer common.BufPoolUdp.Put(buf)

			s.task.Flow.Add(int64(len(data)), 0)
			for {
				clientConn.SetReadDeadline(time.Now().Add(time.Minute * 10))
				if n, err := target.Read(buf); err != nil {
					s.addrMap.Delete(addr.String())
					logs.Warn(err)
					return
				} else {
					s.listener.WriteTo(buf[:n], addr)
					s.task.Flow.Add(0, int64(n))
				}
			}
		}
	}
}

func (s *UdpModeServer) Close() error {
	return s.listener.Close()
}
