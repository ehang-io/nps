package common

import (
	"net"
	"testing"
	"time"
)

// 半关闭必须真的只关写方向：对端要能读到 EOF，而本端还能继续接收数据。
// 这一条直接决定 21/22/80 这类协议收尾时响应会不会被截断。
func TestCloseWriteKeepsReadSide(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	serverGot := make(chan string, 1)
	serverEOF := make(chan struct{}, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 16)
		n, _ := c.Read(buf)
		serverGot <- string(buf[:n])
		// 继续读：对端半关闭后这里应当拿到 EOF
		if _, err := c.Read(buf); err != nil {
			serverEOF <- struct{}{}
		}
		// 半关闭只影响对端的写方向，服务端仍可以把响应发回去
		c.Write([]byte("BYE"))
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	TuneConn(client)
	if _, err := client.Write([]byte("PING")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !CloseWrite(client) {
		t.Fatal("CloseWrite not supported on tuned tcp conn")
	}

	if got := <-serverGot; got != "PING" {
		t.Fatalf("server read %q, want PING", got)
	}
	select {
	case <-serverEOF:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not observe EOF after CloseWrite")
	}

	// 半关闭之后本端仍然可读
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 8)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read after CloseWrite: %v", err)
	}
	if string(buf[:n]) != "BYE" {
		t.Fatalf("read %q, want BYE", string(buf[:n]))
	}
}

func TestCloseWriteOnNonTCP(t *testing.T) {
	// 不支持半关闭的连接返回 false 而不是 panic
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	if CloseWrite(left) {
		t.Fatal("net.Pipe should not support CloseWrite")
	}
	if CloseWrite(nil) {
		t.Fatal("nil should not support CloseWrite")
	}
}

func TestTuneConnNil(t *testing.T) {
	if TuneConn(nil) != nil {
		t.Fatal("TuneConn(nil) should return nil")
	}
}

func TestDialTuned(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			c.Close()
		}
	}()

	c, err := DialTuned("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("DialTuned: %v", err)
	}
	defer c.Close()
	if _, ok := c.(*net.TCPConn); !ok {
		t.Fatalf("DialTuned returned %T, want *net.TCPConn", c)
	}

	// 目标不可达时必须在超时内返回，不能一直挂着
	start := time.Now()
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	deadAddr := ln2.Addr().String()
	ln2.Close()
	if _, err := DialTuned("tcp", deadAddr, 300*time.Millisecond); err == nil {
		t.Fatal("expected error dialing closed port")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("dial took too long: %s", time.Since(start))
	}
}
