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
	clientConn net.Conn
	ctx        context.Context
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
)

func (request *Request) Run(ctx context.Context) (context.Context, error) {
	request.clientConn = request.GetClientConn(ctx)
	request.ctx = ctx

	/*
		The SOCKS request is formed as follows:
		+----+-----+-------+------+----------+----------+
		|VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
		+----+-----+-------+------+----------+----------+
		| 1  |  1  | X'00' |  1   | Variable |    2     |
		+----+-----+-------+------+----------+----------+
	*/
	header := make([]byte, 3)

	_, err := io.ReadFull(request.clientConn, header)

	if err != nil {
		return request.ctx, errors.New("illegal request" + err.Error())
	}

	switch header[1] {
	case connectMethod:
		request.ctx = context.WithValue(request.ctx, core.PROXY_CONNECTION_TYPE, "tcp")
		return request.ctx, request.doConnect()
	case bindMethod:
		return request.ctx, request.handleBind()
	case associateMethod:
		request.ctx = context.WithValue(request.ctx, core.PROXY_CONNECTION_TYPE, "udp")
		return request.ctx, request.handleUDP()
	default:
		request.sendReply(commandNotSupported)
		return request.ctx, errors.New("command not supported")
	}
	return request.ctx, nil
}

func (request *Request) sendReply(rep uint8) error {
	reply := []byte{
		5,
		rep,
		0,
		1,
	}
	localAddr := request.clientConn.LocalAddr().String()
	localHost, localPort, _ := net.SplitHostPort(localAddr)
	ipBytes := net.ParseIP(localHost).To4()
	nPort, _ := strconv.Atoi(localPort)
	reply = append(reply, ipBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(nPort))
	reply = append(reply, portBytes...)
	_, err := request.clientConn.Write(reply)
	return err
}

//do conn
func (request *Request) doConnect() error {
	addrType := make([]byte, 1)
	request.clientConn.Read(addrType)

	var host string
	switch addrType[0] {
	case ipV4:
		ipv4 := make(net.IP, net.IPv4len)
		request.clientConn.Read(ipv4)
		host = ipv4.String()
	case ipV6:
		ipv6 := make(net.IP, net.IPv6len)
		request.clientConn.Read(ipv6)
		host = ipv6.String()
	case domainName:
		var domainLen uint8
		binary.Read(request.clientConn, binary.BigEndian, &domainLen)
		domain := make([]byte, domainLen)
		request.clientConn.Read(domain)
		host = string(domain)
	default:
		request.sendReply(addrTypeNotSupported)
		return errors.New("target address type is not support")
	}

	var port uint16
	binary.Read(request.clientConn, binary.BigEndian, &port)

	request.ctx = context.WithValue(request.ctx, core.PROXY_CONNECTION_ADDR, host)
	request.ctx = context.WithValue(request.ctx, core.PROXY_CONNECTION_PORT, port)
	return nil
}

// passive mode
func (request *Request) handleBind() error {
	return nil
}

//udp
func (request *Request) handleUDP() error {
	/*
	   +----+------+------+----------+----------+----------+
	   |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
	   +----+------+------+----------+----------+----------+
	   | 2  |  1   |  1   | Variable |    2     | Variable |
	   +----+------+------+----------+----------+----------+
	*/
	buf := make([]byte, 3)
	request.clientConn.Read(buf)
	// relay udp datagram silently, without any notification to the requesting client
	if buf[2] != 0 {
		// does not support fragmentation, drop it
		dummy := make([]byte, maxUDPPacketSize)
		request.clientConn.Read(dummy)
		return errors.New("does not support fragmentation, drop")
	}
	return request.doConnect()
}
