package mux

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"log"
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
		connCh := make(chan bool, 1)
		for {
			c, err := m2.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			connCh <- true
			go func(c net.Conn, ch chan bool) {
				c2, err := net.Dial("tcp", "127.0.0.1:80")
				if err != nil {
					log.Fatalln(err)
				}
				go common.CopyBuffer(c2, c)
				common.CopyBuffer(c, c2)
				c2.Close()
				c.Close()
				logs.Warn("close npc")
				<-ch
			}(c, connCh)
		}
	}()

	go func() {
		m1 := NewMux(conn1, "tcp")
		l, err := net.Listen("tcp", "127.0.0.1:7777")
		if err != nil {
			log.Fatalln(err)
		}
		connCh := make(chan bool, 1)
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			connCh <- true
			go func(conn net.Conn, ch chan bool) {
				tmpCpnn, err := m1.NewConn()
				if err != nil {
					log.Fatalln(err)
				}
				go common.CopyBuffer(tmpCpnn, conn)
				common.CopyBuffer(conn, tmpCpnn)
				conn.Close()
				//tmpCpnn.Close()
				logs.Warn("close from out nps ", tmpCpnn.connId)
				<-ch
			}(conn, connCh)
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
		log.Fatalln(err)
	}
	go func() {
		conn1, err = l.Accept()
		if err != nil {
			log.Fatalln(err)
		}
	}()
	return
}

func client() {
	var err error
	conn2, err = net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		log.Fatalln(err)
	}
}

func TestNewConn(t *testing.T) {
	buf := pool.GetBufPoolCopy()
	logs.Warn(len(buf), cap(buf))
	//b := pool.GetBufPoolCopy()
	//b[0] = 1
	//b[1] = 2
	//b[2] = 3
	b := []byte{1, 2, 3}
	logs.Warn(copy(buf[:3], b), len(buf), cap(buf))
	logs.Warn(len(buf), buf[0])
}
