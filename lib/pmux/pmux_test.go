package pmux

import (
	"testing"
	"time"

	"github.com/astaxie/beego/logs"
)

func TestPortMux_Close(t *testing.T) {
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)

	pMux := NewPortMux(8888, "Ds")
	go func() {
		if pMux.Start() != nil {
			logs.Warn("Error")
		}
	}()
	time.Sleep(time.Second * 3)
	go func() {
		l := pMux.GetHttpListener()
		conn, err := l.Accept()
		logs.Warn(conn, err)
	}()
	go func() {
		l := pMux.GetHttpListener()
		conn, err := l.Accept()
		logs.Warn(conn, err)
	}()
	go func() {
		l := pMux.GetHttpListener()
		conn, err := l.Accept()
		logs.Warn(conn, err)
	}()
	l := pMux.GetHttpListener()
	conn, err := l.Accept()
	logs.Warn(conn, err)
}
