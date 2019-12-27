// +build !windows

package mux

import (
	"errors"
	"net"
	"os"
	"syscall"
)

func sysGetSock(fd *os.File) (bufferSize int, err error) {
	return syscall.GetsockoptInt(int(fd.Fd()), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
}

func getConnFd(c net.Conn) (fd *os.File, err error) {
	switch c.(type) {
	case *net.TCPConn:
		fd, err = c.(*net.TCPConn).File()
		if err != nil {
			return
		}
		return
	case *net.UDPConn:
		fd, err = c.(*net.UDPConn).File()
		if err != nil {
			return
		}
		return
	default:
		err = errors.New("mux:unknown conn type, only tcp or kcp")
		return
	}
}
