package conn

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/xtaci/kcp-go"
)

//acceptErrorBackoff 用于在 Accept 持续失败（如 fd 耗尽）时退避，避免空转占满 CPU
const acceptErrorBackoff = 100 * time.Millisecond

func NewTcpListenerAndProcess(addr string, f func(c net.Conn), listener *net.Listener) error {
	var err error
	*listener, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	Accept(*listener, f)
	return nil
}

func NewKcpListenerAndProcess(addr string, f func(c net.Conn)) error {
	kcpListener, err := kcp.ListenWithOptions(addr, nil, 150, 3)
	if err != nil {
		logs.Error(err)
		return err
	}
	for {
		c, err := kcpListener.AcceptKCP()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			logs.Warn(err)
			time.Sleep(acceptErrorBackoff)
			continue
		}
		SetUdpSession(c)
		go f(c)
	}
	return nil
}

func Accept(l net.Listener, f func(c net.Conn)) {
	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "the mux has closed") {
				break
			}
			logs.Warn(err)
			time.Sleep(acceptErrorBackoff)
			continue
		}
		if c == nil {
			logs.Warn("nil connection")
			break
		}
		go f(c)
	}
}
