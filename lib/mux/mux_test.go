package mux

import (
	"github.com/cnlh/nps/lib/common"
	conn3 "github.com/cnlh/nps/lib/conn"
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
		http.ListenAndServe("0.0.0.0:8899", nil)
	}()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	server()
	client()
	time.Sleep(time.Second * 3)
	go func() {
		m2 := NewMux(conn2)
		for {
			c, err := m2.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			go func(c net.Conn) {
				c2, err := net.Dial("tcp", "127.0.0.1:8080")
				if err != nil {
					log.Fatalln(err)
				}
				go common.CopyBuffer(c2, conn3.NewCryptConn(c, true, nil))
				common.CopyBuffer(conn3.NewCryptConn(c, true, nil), c2)
				c.Close()
				c2.Close()
			}(c)
		}
	}()

	go func() {
		m1 := NewMux(conn1)
		l, err := net.Listen("tcp", "127.0.0.1:7777")
		if err != nil {
			log.Fatalln(err)
		}
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			go func(conn net.Conn) {
				tmpCpnn, err := m1.NewConn()
				if err != nil {
					log.Fatalln(err)
				}
				go common.CopyBuffer(conn3.NewCryptConn(tmpCpnn, true, nil), conn)
				common.CopyBuffer(conn, conn3.NewCryptConn(tmpCpnn, true, nil))
				conn.Close()
				tmpCpnn.Close()
			}(conn)
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
