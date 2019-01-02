package lib

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

const (
	ipV4            = 1
	domainName      = 3
	ipV6            = 4
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

const (
	UserPassAuth    = uint8(2)
	userAuthVersion = uint8(1)
	authSuccess     = uint8(0)
	authFailure     = uint8(1)
)

type Sock5ModeServer struct {
	bridge     *Tunnel
	httpPort   int
	u          string //用户名
	p          string //密码
	enCompress int
	deCompress int
	isVerify   bool
	listener   net.Listener
	vKey       string
	crypt      bool
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
	client, err := s.bridge.GetTunnel(getverifyval(s.vKey), s.enCompress, s.deCompress, s.crypt)
	if err != nil {
		log.Println(err)
		client.Close()
		return
	}
	s.sendReply(c, succeeded)
	var ltype string
	if command == associateMethod {
		ltype = CONN_UDP
	} else {
		ltype = CONN_TCP
	}
	_, err = client.WriteHost(ltype, addr)
	return client, nil
}

func (s *Sock5ModeServer) handleConnect(c net.Conn) {
	proxyConn, err := s.doConnect(c, connectMethod)
	if err != nil {
		log.Println(err)
		c.Close()
	} else {
		go relay(proxyConn, NewConn(c), s.enCompress, s.crypt)
		go relay(NewConn(c), proxyConn, s.deCompress, s.crypt)
	}

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
		go relay(proxyConn, NewConn(c), s.enCompress, s.crypt)
		go relay(NewConn(c), proxyConn, s.deCompress, s.crypt)
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
	if s.isVerify {
		buf[1] = UserPassAuth
		c.Write(buf)
		if err := s.Auth(c); err != nil {
			c.Close()
			log.Println("验证失败：", err)
			return
		}
	} else {
		buf[1] = 0
		c.Write(buf)
	}
	s.handleRequest(c)
}

func (s *Sock5ModeServer) Auth(c net.Conn) error {
	header := []byte{0, 0}
	if _, err := io.ReadAtLeast(c, header, 2); err != nil {
		return err
	}
	if header[0] != userAuthVersion {
		return errors.New("验证方式不被支持")
	}
	userLen := int(header[1])
	user := make([]byte, userLen)
	if _, err := io.ReadAtLeast(c, user, userLen); err != nil {
		return err
	}
	if _, err := c.Read(header[:1]); err != nil {
		return errors.New("密码长度获取错误")
	}
	passLen := int(header[0])
	pass := make([]byte, passLen)
	if _, err := io.ReadAtLeast(c, pass, passLen); err != nil {
		return err
	}
	if string(pass) == s.p && string(user) == s.u {
		if _, err := c.Write([]byte{userAuthVersion, authSuccess}); err != nil {
			return err
		}
		return nil
	} else {
		if _, err := c.Write([]byte{userAuthVersion, authFailure}); err != nil {
			return err
		}
		return errors.New("验证不通过")
	}
	return errors.New("未知错误")
}

func (s *Sock5ModeServer) Start() error {
	s.listener, err = net.Listen("tcp", ":"+strconv.Itoa(s.httpPort))
	if err != nil {
		return err
	}
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			log.Fatal("accept error: ", err)
		}
		go s.handleNewConn(conn)
	}
	return nil
}

func (s *Sock5ModeServer) Close() error {
	return s.listener.Close()
}

func NewSock5ModeServer(httpPort int, u, p string, brige *Tunnel, enCompress int, deCompress int, vKey string, crypt bool) *Sock5ModeServer {
	s := new(Sock5ModeServer)
	s.httpPort = httpPort
	s.bridge = brige
	if u != "" && p != "" {
		s.isVerify = true
		s.u = u
		s.p = p
	} else {
		s.isVerify = false
	}
	s.enCompress = enCompress
	s.deCompress = deCompress
	s.vKey = vKey
	s.crypt = crypt
	return s
}
