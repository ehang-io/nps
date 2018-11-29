package main

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
)

const (
	ipV4       = 1
	domainName = 3
	ipV6       = 4
	connectMethod   = 1
	bindMethod      = 2
	associateMethod = 3
	// The maximum packet size of any udp Associate packet, based on ethernet's max size,
	// minus the IP and UDP headers. IPv4 has a 20 byte header, UDP adds an
	// additional 4 bytes.  This is a total overhead of 24 bytes.  Ethernet's
	// max packet size is 1500 bytes,  1500 - 24 = 1476.
	maxUDPPacketSize = 1476
)

const (
	succeeded uint8 = iota
	serverFailure
	notAllowed
	networkUnreachable
	hostUnreachable
	connectionRefused
	ttlExpired
	commandNotSupported
	addrTypeNotSupported
)

type Sock5ModeServer struct {
	Tunnel
	httpPort int
}

func (s *Sock5ModeServer) handleRequest(c net.Conn) {
	/*
		The SOCKS request is formed as follows:
		+----+-----+-------+------+----------+----------+
		|VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
		+----+-----+-------+------+----------+----------+
		| 1  |  1  | X'00' |  1   | Variable |    2     |
		+----+-----+-------+------+----------+----------+
	*/
	header := make([]byte, 3)

	_, err := io.ReadFull(c, header)

	if err != nil {
		log.Println("illegal request", err)
		c.Close()
		return
	}

	switch header[1] {
	case connectMethod:
		s.handleConnect(c)
	case bindMethod:
		s.handleBind(c)
	case associateMethod:
		s.handleUDP(c)
	default:
		s.sendReply(c, commandNotSupported)
		c.Close()
	}
}

func (s *Sock5ModeServer) sendReply(c net.Conn, rep uint8) {
	reply := []byte{
		5,
		rep,
		0,
		1,
	}

	localAddr := c.LocalAddr().String()
	localHost, localPort, _ := net.SplitHostPort(localAddr)
	ipBytes := net.ParseIP(localHost).To4()
	nPort, _ := strconv.Atoi(localPort)
	reply = append(reply, ipBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(nPort))
	reply = append(reply, portBytes...)

	c.Write(reply)
}

func (s *Sock5ModeServer) doConnect(c net.Conn, command uint8) (proxyConn *Conn, err error) {
	addrType := make([]byte, 1)
	c.Read(addrType)
	var host string
	switch addrType[0] {
	case ipV4:
		ipv4 := make(net.IP, net.IPv4len)
		c.Read(ipv4)
		host = ipv4.String()
	case ipV6:
		ipv6 := make(net.IP, net.IPv6len)
		c.Read(ipv6)
		host = ipv6.String()
	case domainName:
		var domainLen uint8
		binary.Read(c, binary.BigEndian, &domainLen)
		domain := make([]byte, domainLen)
		c.Read(domain)
		host = string(domain)
	default:
		s.sendReply(c, addrTypeNotSupported)
		err = errors.New("Address type not supported")
		return nil, err
	}

	var port uint16
	binary.Read(c, binary.BigEndian, &port)

	// connect to host
	addr := net.JoinHostPort(host, strconv.Itoa(int(port)))
	//取出一个连接
	if len(s.tunnelList) < 10 { //新建通道
		go s.newChan()
	}
	client := <-s.tunnelList
	s.sendReply(c, succeeded)
	_, err = client.WriteHost(addr)
	return client, nil
}

func (s *Sock5ModeServer) handleConnect(c net.Conn) {
	proxyConn, err := s.doConnect(c, connectMethod)
	if err != nil {
		c.Close()
	} else {
		go io.Copy(c, proxyConn)
		go io.Copy(proxyConn, c)
	}

}

func (s *Sock5ModeServer) relay(in, out net.Conn) {
	if _, err := io.Copy(in, out); err != nil {
		log.Println("copy error", err)
	}
	in.Close() // will trigger an error in the other relay, then call out.Close()
}

// passive mode
func (s *Sock5ModeServer) handleBind(c net.Conn) {
}

func (s *Sock5ModeServer) handleUDP(c net.Conn) {
	log.Println("UDP Associate")
	/*
	   +----+------+------+----------+----------+----------+
	   |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
	   +----+------+------+----------+----------+----------+
	   | 2  |  1   |  1   | Variable |    2     | Variable |
	   +----+------+------+----------+----------+----------+
	*/
	buf := make([]byte, 3)
	c.Read(buf)
	// relay udp datagram silently, without any notification to the requesting client
	if buf[2] != 0 {
		// does not support fragmentation, drop it
		log.Println("does not support fragmentation, drop")
		dummy := make([]byte, maxUDPPacketSize)
		c.Read(dummy)
	}

	proxyConn, err := s.doConnect(c, associateMethod)
	if err != nil {
		c.Close()
	} else {
		go io.Copy(c, proxyConn)
		go io.Copy(proxyConn, c)
	}
}

func (s *Sock5ModeServer) handleNewConn(c net.Conn) {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(c, buf); err != nil {
		log.Println("negotiation err", err)
		c.Close()
		return
	}

	if version := buf[0]; version != 5 {
		log.Println("only support socks5, request from: ", c.RemoteAddr())
		c.Close()
		return
	}
	nMethods := buf[1]

	methods := make([]byte, nMethods)
	if len, err := c.Read(methods); len != int(nMethods) || err != nil {
		log.Println("wrong method")
		c.Close()
		return
	}
	// no authentication required for now
	buf[1] = 0
	// send a METHOD selection message
	c.Write(buf)

	s.handleRequest(c)
}

func (s *Sock5ModeServer) Start() {
	l, err := net.Listen("tcp", ":"+strconv.Itoa(s.httpPort))
	if err != nil {
		log.Fatal("listen error: ", err)
	}
	s.StartTunnel()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("accept error: ", err)
		}
		go s.handleNewConn(conn)
	}
}

func NewSock5ModeServer(tcpPort, httpPort int) *Sock5ModeServer {
	s := new(Sock5ModeServer)
	s.tunnelPort = tcpPort
	s.httpPort = httpPort
	s.tunnelList = make(chan *Conn, 1000)
	s.signalList = make(chan *Conn, 10)
	return s
}
