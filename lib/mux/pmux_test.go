package mux

import (
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"testing"
	"time"
)

func TestPortMux_Close(t *testing.T) {
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)

	pMux := NewPortMux(8888)
	go func() {
		if pMux.Start() != nil {
			logs.Warn("Error")
		}
	}()
	time.Sleep(time.Second * 3)
	go func() {
		l := pMux.GetHttpsAccept()
		conn, err := l.Accept()
		logs.Warn(conn, err)
	}()
	go func() {
		l := pMux.GetHttpAccept()
		conn, err := l.Accept()
		logs.Warn(conn, err)
	}()
	go func() {
		l := pMux.GetClientAccept()
		conn, err := l.Accept()
		logs.Warn(conn, err)
	}()
	l := pMux.GetManagerAccept()
	conn, err := l.Accept()
	logs.Warn(conn, err)
}
