// +build !windows

package nps_mux

import (
	"net"
	"syscall"
)

func sysGetSock(raw syscall.RawConn) (bufferSize int, err error) {
	if raw == nil {
		return 5 * 1024 * 1024, nil
	}
	if cerr := raw.Control(func(fd uintptr) {
		bufferSize, err = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
	}); cerr != nil {
		return 0, cerr
	}
	return
}

// getRawConn gives access to the socket without duplicating it. It used to
// take a File(): that dup was never closed, so every closed mux left its
// socket open (CLOSE-WAIT once the peer had gone) until a finaliser ran, and
// reading through it switched the connection to blocking mode, which turns
// off its deadlines.
func getRawConn(c net.Conn) (syscall.RawConn, error) {
	c = underlying(c)
	switch c.(type) {
	case *net.TCPConn, *net.UDPConn:
		return c.(syscall.Conn).SyscallConn()
	default:
		return nil, nil
	}
}
