package socks5

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/cnlh/nps/core"
	"io"
	"net"
	"strconv"
)

type Request struct {
	core.NpsPlugin
}

const (
	ipV4            = 1
	domainName      = 3
	ipV6            = 4
	connectMethod   = 1
	bindMethod      = 2
	associateMethod = 3
	// The maximum packet size of any udp Associate packet, based on ethernet's max size,
	// minus the IP and UDP headerrequest. IPv4 has a 20 byte header, UDP adds an
	// additional 4 byterequest.  This is a total overhead of 24 byterequest.  Ethernet's
	// max packet size is 1500 bytes,  1500 - 24 = 1476.
	maxUDPPacketSize     = 1476
	commandNotSupported  = 7
	addrTypeNotSupported = 8
	succeeded            = 0
)

func (request *Request) Run(ctx context.Context) (context.Context, error) {
	clientConn := request.GetClientConn(ctx)

	/*
		The SOCKS request is formed as follows:
		+----+-----+-------+------+----------+----------+
		|VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
		+----+-----+-------+------+----------+----------+
		| 1  |  1  | X'00' |  1   | Variable |    2     |
		+----+-----+-------+------+----------+----------+
	*/
	header := make([]byte, 3)

	_, err := io.ReadFull(clientConn, header)

	if err != nil {
		return ctx, errors.New("illegal request" + err.Error())
	}

	switch header[1] {
	case connectMethod:
		ctx = context.WithValue(ctx, core.PROXY_CONNECTION_TYPE, "tcp")
		return request.doConnect(ctx, clientConn)
	case bindMethod:
		return ctx, request.handleBind()
	case associateMethod:
		ctx = context.WithValue(ctx, core.PROXY_CONNECTION_TYPE, "udp")
		return request.handleUDP(ctx, clientConn)
	default:
		request.sendReply(clientConn, commandNotSupported)
		return ctx, errors.New("command not supported")
	}
	return ctx, nil
}

func (request *Request) sendReply(clientConn net.Conn, rep uint8) error {
	reply := []byte{
		5,
		rep,
		0,
		1,
	}
	localAddr := clientConn.LocalAddr().String()
	localHost, localPort, _ := net.SplitHostPort(localAddr)
	ipBytes := net.ParseIP(localHost).To4()
	nPort, _ := strconv.Atoi(localPort)
	reply = append(reply, ipBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(nPort))
	reply = append(reply, portBytes...)
	_, err := clientConn.Write(reply)
	return err
}

//do conn
func (request *Request) doConnect(ctx context.Context, clientConn net.Conn) (context.Context, error) {
	addrType := make([]byte, 1)
	clientConn.Read(addrType)

	var host string
	switch addrType[0] {
	case ipV4:
		ipv4 := make(net.IP, net.IPv4len)
		clientConn.Read(ipv4)
		host = ipv4.String()
	case ipV6:
		ipv6 := make(net.IP, net.IPv6len)
		clientConn.Read(ipv6)
		host = ipv6.String()
	case domainName:
		var domainLen uint8
		binary.Read(clientConn, binary.BigEndian, &domainLen)
		domain := make([]byte, domainLen)
		clientConn.Read(domain)
		host = string(domain)
	default:
		request.sendReply(clientConn, addrTypeNotSupported)
		return ctx, errors.New("target address type is not support")
	}
	var port uint16
	binary.Read(clientConn, binary.BigEndian, &port)
	ctx = context.WithValue(ctx, core.PROXY_CONNECTION_ADDR, host)
	ctx = context.WithValue(ctx, core.PROXY_CONNECTION_PORT, port)
	request.sendReply(clientConn, succeeded)
	return ctx, nil
}

// passive mode
func (request *Request) handleBind() error {
	return nil
}

//udp
func (request *Request) handleUDP(ctx context.Context, clientConn net.Conn) (context.Context, error) {
	/*
	   +----+------+------+----------+----------+----------+
	   |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
	   +----+------+------+----------+----------+----------+
	   | 2  |  1   |  1   | Variable |    2     | Variable |
	   +----+------+------+----------+----------+----------+
	*/
	buf := make([]byte, 3)
	clientConn.Read(buf)
	// relay udp datagram silently, without any notification to the requesting client
	if buf[2] != 0 {
		// does not support fragmentation, drop it
		dummy := make([]byte, maxUDPPacketSize)
		clientConn.Read(dummy)
		return ctx, errors.New("does not support fragmentation, drop")
	}
	return request.doConnect(ctx, clientConn)
}
