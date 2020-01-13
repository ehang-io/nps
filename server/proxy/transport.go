// +build !windows

package proxy

import (
	"net"
	"strconv"
	"syscall"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
)

func HandleTrans(c *conn.Conn, s *TunnelModeServer) error {
	if addr, err := getAddress(c.Conn); err != nil {
		return err
	} else {
		return s.DealClient(c, s.task.Client, addr, nil, common.CONN_TCP, nil, s.task.Flow, s.task.Target.LocalProxy)
	}
}

const SO_ORIGINAL_DST = 80

func getAddress(conn net.Conn) (string, error) {
	sysrawconn, f := conn.(syscall.Conn)
	if !f {
		return "", nil
	}
	rawConn, err := sysrawconn.SyscallConn()
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
	return ip + ":" + strconv.Itoa(int(port)), nil
}
