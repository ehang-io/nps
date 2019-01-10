package server

import (
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"io"
	"log"
	"net"
	"strings"
)

type UdpModeServer struct {
	bridge   *bridge.Tunnel
	listener *net.UDPConn
	udpMap   map[string]*utils.Conn
	config   *ServerConfig
}

func NewUdpModeServer(bridge *bridge.Tunnel, cnf *ServerConfig) *UdpModeServer {
	s := new(UdpModeServer)
	s.bridge = bridge
	s.udpMap = make(map[string]*utils.Conn)
	s.config = cnf
	return s
}

//开始
func (s *UdpModeServer) Start() error {
	var err error
	s.listener, err = net.ListenUDP("udp", &net.UDPAddr{net.ParseIP("0.0.0.0"), s.config.TcpPort, ""})
	if err != nil {
		return err
	}
	data := make([]byte, 1472) //udp数据包大小
	for {
		n, addr, err := s.listener.ReadFromUDP(data)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			continue
		}
		go s.process(addr, data[:n])
	}
	return nil
}

//TODO:效率问题有待解决
func (s *UdpModeServer) process(addr *net.UDPAddr, data []byte) {
	conn, err := s.bridge.GetTunnel(getverifyval(s.config.VerifyKey), s.config.CompressEncode, s.config.CompressDecode, s.config.Crypt, s.config.Mux)
	if err != nil {
		log.Println(err)
		return
	}
	if _, err := conn.WriteHost(utils.CONN_UDP, s.config.Target); err != nil {
		conn.Close()
		return
	}
	if flag, err := conn.ReadFlag(); err == nil {
		defer func() {
			if s.config.Mux {
				conn.WriteTo([]byte(utils.IO_EOF), s.config.CompressEncode, s.config.Crypt)
				s.bridge.ReturnTunnel(conn, getverifyval(s.config.VerifyKey))
			} else {
				conn.Close()
			}
		}()
		if flag == utils.CONN_SUCCESS {
			conn.WriteTo(data, s.config.CompressEncode, s.config.Crypt)
			buf := make([]byte, 1024)
			//conn.conn.SetReadDeadline(time.Now().Add(time.Duration(time.Second * 3)))
			n, err := conn.ReadFrom(buf, s.config.CompressDecode, s.config.Crypt)
			if err != nil || err == io.EOF {
				log.Println("revieve error:", err)
				return
			}
			s.listener.WriteToUDP(buf[:n], addr)
		}
	}
}

func (s *UdpModeServer) Close() error {
	return s.listener.Close()
}
