package mux

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"net"
	"net/http"
	_ "net/http/pprof"
	"testing"
	"time"
)

var conn1 net.Conn
var conn2 net.Conn

func TestNewMux(t *testing.T) {
	go func() {
		http.ListenAndServe("0.0.0.0:8889", nil)
	}()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	server()
	client()
	time.Sleep(time.Second * 3)
	go func() {
		m2 := NewMux(conn2, "tcp")
		for {
			logs.Warn("npc starting accept")
			c, err := m2.Accept()
			if err != nil {
				logs.Warn(err)
				continue
			}
			logs.Warn("npc accept success ")
			c2, err := net.Dial("tcp", "127.0.0.1:80")
			if err != nil {
				logs.Warn(err)
				continue
			}
			var npcToServer common.ConnCopy
			npcToServer.New(c2, c, 0)
			go npcToServer.CopyConn()
			var serverToNpc common.ConnCopy
			serverToNpc.New(c, c2, 10000)
			_, err = serverToNpc.CopyConn()
			if err == nil {
				logs.Warn("close npc")
				c2.Close()
				c.Close()
			}
		}
	}()

	go func() {
		m1 := NewMux(conn1, "tcp")
		l, err := net.Listen("tcp", "127.0.0.1:7777")
		if err != nil {
			logs.Warn(err)
		}
		for {
			logs.Warn("nps starting accept")
			conn, err := l.Accept()
			if err != nil {
				logs.Warn(err)
				continue
			}
			logs.Warn("nps accept success starting new conn")
			tmpCpnn, err := m1.NewConn()
			if err != nil {
				logs.Warn("nps new conn err ", err)
				continue
			}
			logs.Warn("nps new conn success ", tmpCpnn.connId)
			var userToNps common.ConnCopy
			userToNps.New(tmpCpnn, conn, tmpCpnn.connId)
			go userToNps.CopyConn()
			var npsToUser common.ConnCopy
			npsToUser.New(conn, tmpCpnn, tmpCpnn.connId+10000)
			_, err = npsToUser.CopyConn()
			if err == nil {
				logs.Warn("close from out nps ", tmpCpnn.connId)
				conn.Close()
				tmpCpnn.Close()
			}
		}
	}()

	for {
		time.Sleep(time.Second * 5)
	}
}

func server() {
	var err error
	l, err := net.Listen("tcp", "127.0.0.1:9999")
	if err != nil {
		logs.Warn(err)
	}
	go func() {
		conn1, err = l.Accept()
		if err != nil {
			logs.Warn(err)
		}
	}()
	return
}

func client() {
	var err error
	conn2, err = net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		logs.Warn(err)
	}
}

func TestNewConn(t *testing.T) {
	buf := common.GetBufPoolCopy()
	logs.Warn(len(buf), cap(buf))
	//b := pool.GetBufPoolCopy()
	//b[0] = 1
	//b[1] = 2
	//b[2] = 3
	b := []byte{1, 2, 3}
	logs.Warn(copy(buf[:3], b), len(buf), cap(buf))
	logs.Warn(len(buf), buf[0])
}
