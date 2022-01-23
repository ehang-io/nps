package process

import (
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"go.uber.org/zap"
	"net"
	"strconv"
	"syscall"
)

const SO_ORIGINAL_DST = 80

type TransparentProcess struct {
	DefaultProcess
}

func (tp *TransparentProcess) GetName() string {
	return "transparent"
}

func (tp *TransparentProcess) GetZhName() string {
	return "透明代理"
}

func (tp *TransparentProcess) ProcessConn(c enet.Conn) (bool, error) {
	addr, err := tp.getAddress(c)
	if err != nil {
		logger.Debug("get syscall error", zap.Error(err))
		return false, nil
	}
	return true, tp.ac.RunConnWithAddr(c, addr)
}

func (tp *TransparentProcess) getAddress(conn net.Conn) (string, error) {
	// TODO: IPV6 support
	sysrawConn, f := conn.(syscall.Conn)
	if !f {
		return "", nil
	}
	rawConn, err := sysrawConn.SyscallConn()
	if err != nil {
		return "", nil
	}
	var ip string
	var port uint16
	err = rawConn.Control(func(fd uintptr) {
		addr, err := syscall.GetsockoptIPv6Mreq(int(fd), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
		if err != nil {
			return
		}
		ip = net.IP(addr.Multiaddr[4:8]).String()
		port = uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])
	})
	return net.JoinHostPort(ip, strconv.Itoa(int(port))), nil
}
