package lib

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type UdpModeServer struct {
	bridge       *Tunnel
	udpPort      int    //监听的udp端口
	tunnelTarget string //udp目标地址
	listener     *net.UDPConn
	udpMap       map[string]*Conn
	enCompress   int
	deCompress   int
	vKey         string
}

func NewUdpModeServer(udpPort int, tunnelTarget string, bridge *Tunnel, enCompress int, deCompress int, vKey string) *UdpModeServer {
	s := new(UdpModeServer)
	s.udpPort = udpPort
	s.tunnelTarget = tunnelTarget
	s.bridge = bridge
	s.udpMap = make(map[string]*Conn)
	s.enCompress = enCompress
	s.deCompress = deCompress
	s.vKey = vKey
	return s
}

//开始
func (s *UdpModeServer) Start() error {
	s.listener, err = net.ListenUDP("udp", &net.UDPAddr{net.ParseIP("0.0.0.0"), s.udpPort, ""})
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
			log.Println(err)
			continue
		}
		go s.process(addr, data[:n])
	}
	return nil
}

//TODO:效率问题有待解决
func (s *UdpModeServer) process(addr *net.UDPAddr, data []byte) {
	fmt.Println(addr.String())
	fmt.Println(string(data))
	conn, err := s.bridge.GetTunnel(getverifyval(s.vKey), s.enCompress, s.deCompress)
	if err != nil {
		log.Println(err)
		return
	}
	if _, err := conn.WriteHost(CONN_UDP, s.tunnelTarget); err != nil {
		conn.Close()
		return
	}
	conn.WriteCompress(data, s.enCompress)
	go func(addr *net.UDPAddr, conn *Conn) {
		buf := make([]byte, 1024)
		conn.conn.SetReadDeadline(time.Now().Add(time.Duration(time.Second * 3)))
		n, err := conn.ReadFromCompress(buf, s.deCompress)
		if err != nil || err == io.EOF {
			conn.Close()
			return
		}
		s.listener.WriteToUDP(buf[:n], addr)
		conn.Close()
	}(addr, conn)
}

func (s *UdpModeServer) Close() error {
	return s.listener.Close()
}
