package enet

import (
	"math/rand"
	"net"
	"testing"
	"time"
)

func TestReaderConn_Read(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:61254")
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 33*1024)
	go func() {
		conn, err := net.Dial("tcp", "127.0.0.1:61254")
		if err != nil {
			t.Fatal(err)
		}
		rand.Seed(time.Now().UnixNano())
		for i := 0; i < 33*1024; i++ {
			b[i] = byte(rand.Intn(128))
		}
		conn.Write(b)
	}()
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	rConn := NewReaderConn(conn)
	buf := make([]byte, 1024)
	nn := 0
	times := 0
	for {
		n, err := rConn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 1024; i++ {
			if b[times*1024+i] != buf[i] {
				t.Fatal("data error")
			}
		}
		times++
		nn += n
		if nn > 30*1024 {
			break
		}
		if times > 100 {
			t.Fatal("read error")
		}
	}

	rConn.Reset(0)
	nn = 0
	times = 0
	for {
		n, err := rConn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 1024; i++ {
			if b[times*1024+i] != buf[i] {
				t.Fatal("data error")
			}
		}
		nn += n
		times++
		if nn > 32*1024 {
			break
		}
		if times > 100 {
			t.Fatal("read error")
		}
	}
	if !rConn.hasClear || rConn.hasRead != rConn.nowIndex || rConn.nowIndex != MaxReadSize {
		t.Fatal("read error")
	}

}
