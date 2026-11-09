package common

import (
	"errors"
	"net"
	"testing"
	"time"
)

// 3389/22/80/443 都是客户端先说话，应该等首包；21 是服务端先发 220，必须跳过。
func TestProfileForPortKnownPorts(t *testing.T) {
	cases := []struct {
		port        int
		name        string
		serverFirst bool
		waitFirst   bool
	}{
		{3389, "rdp", false, true},
		{22, "ssh", false, true},
		{21, "ftp", true, false},
		{80, "http", false, true},
		{443, "https", false, true},
	}
	for _, c := range cases {
		p := ProfileForPort(c.port)
		if !p.Known {
			t.Fatalf("port %d should be known", c.port)
		}
		if p.Name != c.name {
			t.Fatalf("port %d name = %q, want %q", c.port, p.Name, c.name)
		}
		if p.ServerFirst != c.serverFirst {
			t.Fatalf("port %d ServerFirst = %v, want %v", c.port, p.ServerFirst, c.serverFirst)
		}
		if (p.FirstByteTimeout > 0) != c.waitFirst {
			t.Fatalf("port %d FirstByteTimeout = %v, unexpected", c.port, p.FirstByteTimeout)
		}
		if p.UserTimeout <= 0 {
			t.Fatalf("port %d should have a positive user timeout", c.port)
		}
	}
}

// 未收录端口不能瞎猜协议：不等首包，但仍要有失效连接回收时间。
func TestProfileForPortUnknown(t *testing.T) {
	p := ProfileForPort(45678)
	if p.Known {
		t.Fatal("port 45678 should not be known")
	}
	if p.FirstByteTimeout != 0 || p.ServerFirst {
		t.Fatalf("unknown port profile = %+v, want no first byte wait", p)
	}
	if p.UserTimeout <= 0 {
		t.Fatal("unknown port should still get a default user timeout")
	}
}

// 正常客户端：首包立即到达，必须原样回放给后续读取方。
func TestAwaitFirstByteReplaysData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		_, _ = client.Write([]byte("SSH-2.0-OpenSSH_8.9\r\n"))
	}()

	c, err := AwaitFirstByte(server, time.Second)
	if err != nil {
		t.Fatalf("AwaitFirstByte: %v", err)
	}
	buf := make([]byte, 32)
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read after guard: %v", err)
	}
	if string(buf[:n]) != "SSH-2.0-OpenSSH_8.9\r\n" {
		t.Fatalf("replayed %q", buf[:n])
	}
}

// 扫描器：连上但不发数据，必须按超时判为 ErrNoFirstByte。
func TestAwaitFirstByteTimeout(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	start := time.Now()
	if _, err := AwaitFirstByte(server, 100*time.Millisecond); !errors.Is(err, ErrNoFirstByte) {
		t.Fatalf("err = %v, want ErrNoFirstByte", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("guard took %v, expected ~100ms", elapsed)
	}
}

// 超时时间传 0 表示不启用，连接原样返回。
func TestAwaitFirstByteDisabled(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	c, err := AwaitFirstByte(server, 0)
	if err != nil {
		t.Fatalf("AwaitFirstByte: %v", err)
	}
	if c != server {
		t.Fatal("disabled guard should return the original conn")
	}
}

// 首包判定结束后必须清掉读 deadline，否则正常长连接会在超时后被误杀。
func TestAwaitFirstByteClearsDeadline(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() { _, _ = client.Write([]byte("A")) }()
	c, err := AwaitFirstByte(server, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("AwaitFirstByte: %v", err)
	}
	buf := make([]byte, 8)
	if n, err := c.Read(buf); err != nil || string(buf[:n]) != "A" {
		t.Fatalf("replay = %q, %v", buf[:n], err)
	}
	//等到超过首包超时时间之后再让对方发数据：deadline 若没清除，这里会读到 timeout
	time.Sleep(300 * time.Millisecond)
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = client.Write([]byte("B"))
	}()
	if _, err := c.Read(buf); err != nil {
		t.Fatalf("read after deadline clear: %v", err)
	}
}
