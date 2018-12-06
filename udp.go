package main

import (
	"io"
	"log"
	"net"
	"time"
)

type UdpModeServer struct {
	Tunnel
	udpPort      int    //监听的udp端口
	tunnelTarget string //udp目标地址
	listener     *net.UDPConn
	udpMap       map[string]*Conn
}

func NewUdpModeServer(tcpPort, udpPort int, tunnelTarget string) *UdpModeServer {
	s := new(UdpModeServer)
	s.tunnelPort = tcpPort
	s.udpPort = udpPort
	s.tunnelTarget = tunnelTarget
	s.tunnelList = make(chan *Conn, 1000)
	s.signalList = make(chan *Conn, 10)
	s.udpMap = make(map[string]*Conn)
	return s
}

//开始
func (s *UdpModeServer) Start() (error) {
	err := s.StartTunnel()
	if err != nil {
		log.Fatalln("启动失败!", err)
		return err
	}
	s.startTunnelServer()
	return nil
}

//udp监听
func (s *UdpModeServer) startTunnelServer() {
	s.listener, err = net.ListenUDP("udp", &net.UDPAddr{net.ParseIP("0.0.0.0"), s.udpPort, ""})
	if err != nil {
		log.Fatalln(err)
	}
	data := make([]byte, 1472) //udp数据包大小
	for {
		n, addr, err := s.listener.ReadFromUDP(data)
		if err != nil {
			log.Println(err)
			continue
		}
		go s.process(addr, data[:n])
	}
}

func (s *UdpModeServer) process(addr *net.UDPAddr, data []byte) {
	conn := s.GetTunnel()
	if _, err := conn.WriteHost(CONN_UDP, s.tunnelTarget);err!=nil{
		conn.Close()
		return
	}
	go func() {
		for {
			buf := make([]byte, 1024)
			conn.conn.SetReadDeadline(time.Now().Add(time.Duration(time.Second * 3)))
			n, err := conn.ReadFromCompress(buf, DataDecode)
			if err != nil || err == io.EOF {
				conn.Close()
				break
			}
			_, err = s.listener.WriteToUDP(buf[:n], addr)
			if err != nil {
				conn.Close()
				break
			}
		}

	}()
	if _, err = conn.WriteCompress(data, DataEncode); err != nil {
		conn.Close()
	}
}
