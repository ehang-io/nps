package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"encoding/binary"
	"github.com/pkg/errors"
	"github.com/robfig/go-cache"
	"go.uber.org/zap"
	"io"
	"net"
	"strconv"
	"time"
)

type Socks5Process struct {
	DefaultProcess
	Accounts map[string]string `json:"accounts" placeholder:"username1 password1\nusername2 password2" zh_name:"授权账号密码"`
	ServerIp string            `json:"server_ip" placeholder:"123.123.123.123" zh_name:"udp连接地址"`
	ipStore  *cache.Cache
}

const (
	ipV4            = 1
	domainName      = 3
	ipV6            = 4
	connectMethod   = 1
	bindMethod      = 2
	associateMethod = 3
	// The maximum packet size of any udp Associate packet, based on ethernet's max size,
	// minus the IP and UDP headers5. IPv4 has a 20 byte header, UDP adds an
	// additional 4 bytes5.  This is a total overhead of 24 bytes5.  Ethernet's
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

func (s5 *Socks5Process) GetName() string {
	return "socks5"
}

func (s5 *Socks5Process) GetZhName() string {
	return "socks5代理"
}

func (s5 *Socks5Process) Init(ac action.Action) error {
	s5.ipStore = cache.New(time.Minute, time.Minute*2)
	return s5.DefaultProcess.Init(ac)
}

func (s5 *Socks5Process) ProcessConn(c enet.Conn) (bool, error) {
	return true, s5.handleConn(c)
}

func (s5 *Socks5Process) ProcessPacketConn(pc enet.PacketConn) (bool, error) {
	ip, _, _ := net.SplitHostPort(pc.LocalAddr().String())
	if _, ok := s5.ipStore.Get(ip); !ok {
		return false, nil
	}
	_, addr, err := pc.FirstPacket()
	if err != nil {
		return false, errors.New("addr not found")
	}
	return true, s5.ac.RunPacketConn(enet.NewS5PacketConn(pc, addr))
}

func (s5 *Socks5Process) handleConn(c enet.Conn) error {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(c, buf); err != nil {
		return err
	}

	if version := buf[0]; version != 5 {
		return errors.New("only support socks5")
	}
	nMethods := buf[1]

	methods := make([]byte, nMethods)
	if l, err := c.Read(methods); l != int(nMethods) || err != nil {
		return errors.New("wrong method")
	}

	if len(s5.Accounts) > 0 {
		buf[1] = UserPassAuth
		_, err := c.Write(buf)
		if err != nil {
			return err
		}
		if err := s5.Auth(c); err != nil {
			return errors.Wrap(err, "auth failed")
		}
	} else {
		buf[1] = 0
		_, _ = c.Write(buf)
	}
	return s5.handleRequest(c)
}

func (s5 *Socks5Process) Auth(c enet.Conn) error {
	header := []byte{0, 0}
	if _, err := io.ReadAtLeast(c, header, 2); err != nil {
		return err
	}
	if header[0] != userAuthVersion {
		return errors.New("auth type not support")
	}
	userLen := int(header[1])
	user := make([]byte, userLen)
	if _, err := io.ReadAtLeast(c, user, userLen); err != nil {
		return err
	}
	if _, err := c.Read(header[:1]); err != nil {
		return errors.New("the length of password is incorrect")
	}
	passLen := int(header[0])
	pass := make([]byte, passLen)
	if _, err := io.ReadAtLeast(c, pass, passLen); err != nil {
		return err
	}

	p := s5.Accounts[string(user)]

	if p == "" || string(pass) != p {
		_, _ = c.Write([]byte{userAuthVersion, authFailure})
		return errors.New("auth failure")
	}

	if _, err := c.Write([]byte{userAuthVersion, authSuccess}); err != nil {
		return errors.Wrap(err, "write auth success")
	}
	return nil
}

func (s5 *Socks5Process) handleRequest(c enet.Conn) error {
	header := make([]byte, 3)

	_, err := io.ReadFull(c, header)

	if err != nil {
		return err
	}

	switch header[1] {
	case connectMethod:
		s5.handleConnect(c)
	case associateMethod:
		s5.handleUDP(c)
	default:
		s5.sendReply(c, commandNotSupported)
		c.Close()
	}
	return nil
}

//enet
func (s5 *Socks5Process) handleConnect(c enet.Conn) {
	addr, err := common.ReadAddr(c)
	if err != nil {
		s5.sendReply(c, addrTypeNotSupported)
		logger.Warn("read socks addr error", zap.Error(err))
		return
	}
	s5.sendReply(c, succeeded)
	_ = s5.ac.RunConnWithAddr(c, addr.String())
	return
}

func (s5 *Socks5Process) handleUDP(c net.Conn) {
	_, err := common.ReadAddr(c)
	if err != nil {
		s5.sendReply(c, addrTypeNotSupported)
		logger.Warn("read socks addr error", zap.Error(err))
		return
	}
	ip, _, _ := net.SplitHostPort(c.RemoteAddr().String())
	s5.ipStore.Set(ip, true, time.Minute)
	s5.sendReply(c, succeeded)
}

func (s5 *Socks5Process) sendReply(c net.Conn, rep uint8) {
	reply := []byte{
		5,
		rep,
		0,
		1,
	}

	localHost, localPort, _ := net.SplitHostPort(c.LocalAddr().String())
	if s5.ServerIp != "" {
		localHost = s5.ServerIp
	}
	ipBytes := net.ParseIP(localHost).To4()
	nPort, _ := strconv.Atoi(localPort)
	reply = append(reply, ipBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(nPort))
	reply = append(reply, portBytes...)
	_, _ = c.Write(reply)
}
