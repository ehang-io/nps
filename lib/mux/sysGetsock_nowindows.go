// +build !windows

package mux

import (
	"errors"
	"github.com/xtaci/kcp-go"
	"net"
	"os"
	"syscall"
)

func sysGetSock(fd *os.File) (bufferSize int, err error) {
	if fd != nil {
		return syscall.GetsockoptInt(int(fd.Fd()), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
	} else {
		return 5 * 1024 * 1024, nil
	}
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
	case *kcp.UDPSession:
		//fd, err = (*net.UDPConn)(unsafe.Pointer(c.(*kcp.UDPSession))).File()
		//if err != nil {
		//	return
		//}
		// Todo
		return
	default:
		err = errors.New("mux:unknown conn type, only tcp or kcp")
		return
	}
}
