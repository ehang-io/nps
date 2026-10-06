package nps_mux

import (
	"net"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

func tcpPair(t *testing.T) (client, server *net.TCPConn) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := l.Accept()
		accepted <- c
	}()
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	s := <-accepted
	if s == nil {
		t.Fatal("accept failed")
	}
	t.Cleanup(func() {
		_ = c.Close()
		_ = s.Close()
	})
	return c.(*net.TCPConn), s.(*net.TCPConn)
}

func sockopt(t *testing.T, c *net.TCPConn, get func(fd int) error) {
	raw, err := c.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Control(func(fd uintptr) {
		if err := get(int(fd)); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTuneTCPSetsOptions(t *testing.T) {
	// reno is built into every kernel, so this checks the option is applied
	// without depending on tcp_bbr being loaded on the test machine.
	old, had := os.LookupEnv(congestionEnv)
	_ = os.Setenv(congestionEnv, "reno")
	defer func() {
		if had {
			_ = os.Setenv(congestionEnv, old)
		} else {
			_ = os.Unsetenv(congestionEnv)
		}
	}()

	c, _ := tcpPair(t)
	tuneTCP(c)
	sockopt(t, c, func(fd int) error {
		lowat, err := syscall.GetsockoptInt(fd, syscall.IPPROTO_TCP, tcpNotsentLowat)
		if err != nil {
			return err
		}
		if lowat != notsentLowat {
			t.Errorf("TCP_NOTSENT_LOWAT is %d, want %d", lowat, notsentLowat)
		}
		buf := make([]byte, 16)
		n := uint32(len(buf))
		if _, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, uintptr(fd), syscall.IPPROTO_TCP, syscall.TCP_CONGESTION,
			uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0); errno != 0 {
			return errno
		}
		if got := string(buf[:clen(buf)]); got != "reno" {
			t.Errorf("congestion control is %q, want reno", got)
		}
		return nil
	})
}

func clen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}
