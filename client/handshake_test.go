package client

import (
	"crypto/tls"
	"io"
	"net"
	"strings"
	"testing"

	"ehang.io/nps/lib/bridgetls"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/version"
)

// fakeBridge is the server side of Bridge.cliProcess: TLS first, then one
// hello read field by field, a proof checked against the key it holds, and a
// verdict with its own proof. mutate lets a test make it misbehave.
type fakeBridge struct {
	addr string
	work chan string
}

type behaviour struct {
	vkey     string
	protocol string
	// what a man in the middle or a broken server might do
	verdict     string // empty: success
	wrongServer bool   // answer with a proof made with the wrong key
	echo        bool
}

func startFake(t *testing.T, b behaviour) *fakeBridge {
	cfg, err := bridgetls.ServerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if b.protocol == "" {
		b.protocol = version.Protocol
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	f := &fakeBridge{addr: l.Addr().String(), work: make(chan string, 1)}
	go func() {
		raw, err := l.Accept()
		if err != nil {
			return
		}
		c, err := bridgetls.Server(raw, cfg)
		if err != nil {
			return
		}
		defer c.Close()
		sc := conn.NewConn(c)
		protocol, err := sc.GetShortLenContent()
		if err != nil || string(protocol) != b.protocol {
			t.Errorf("protocol: %q %v", protocol, err)
			return
		}
		vs, err := sc.GetShortLenContent()
		if err != nil || string(vs) != version.VERSION {
			t.Errorf("version: %q %v", vs, err)
			return
		}
		flag, err := sc.ReadFlag()
		if err != nil {
			t.Errorf("work flag: %v", err)
			return
		}
		proof, err := sc.GetShortContent(bridgetls.ProofLen)
		if err != nil {
			t.Errorf("proof: %v", err)
			return
		}
		f.work <- flag
		exporter, _ := bridgetls.Exporter(c)
		hello := bridgetls.Hello(string(protocol), string(vs), flag)
		if b.verdict != "" {
			sc.Write([]byte(b.verdict))
			return
		}
		if !bridgetls.Equal(proof, bridgetls.ClientProof(b.vkey, exporter, hello)) {
			sc.Write([]byte(common.VERIFY_EER))
			return
		}
		key := b.vkey
		if b.wrongServer {
			key = "not-the-vkey"
		}
		sc.Write(append([]byte(common.VERIFY_SUCCESS), bridgetls.ServerProof(key, exporter, proof)...))
		io.Copy(c, c) // echo
	}()
	return f
}

func TestNewConnAuthenticatesBothWays(t *testing.T) {
	f := startFake(t, behaviour{vkey: "vk"})
	c, err := NewConn("tcp", "vk", f.addr, common.WORK_MAIN, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if got := <-f.work; got != common.WORK_MAIN {
		t.Fatalf("server saw work %q", got)
	}
	if _, ok := c.Conn.(*tls.Conn); !ok {
		t.Fatalf("the connection is a %T, want the TLS conn itself", c.Conn)
	}
	c.Write([]byte("hello"))
	buf := make([]byte, 5)
	if _, err := io.ReadFull(c.Conn, buf); err != nil || string(buf) != "hello" {
		t.Fatalf("echo: %q %v", buf, err)
	}
}

func TestNewConnWrongVkeyIsRefusedAndSaysSo(t *testing.T) {
	f := startFake(t, behaviour{vkey: "the-real-one"})
	_, err := NewConn("tcp", "a-wrong-one", f.addr, common.WORK_MAIN, "")
	if err == nil || !strings.Contains(err.Error(), "did not accept") {
		t.Fatalf("a wrong vkey must fail and say the server did not accept it, got %v", err)
	}
}

func TestNewConnProtocolMismatchSaysSo(t *testing.T) {
	f := startFake(t, behaviour{vkey: "vk", verdict: common.VERIFY_PROTOCOL})
	if _, err := NewConn("tcp", "vk", f.addr, common.WORK_MAIN, ""); err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("got %v", err)
	}
}

// A server that cannot prove it holds the vkey is not the server, whatever it
// answers: the client must refuse it and call it out.
func TestNewConnRefusesAServerThatCannotProveItself(t *testing.T) {
	f := startFake(t, behaviour{vkey: "vk", wrongServer: true})
	_, err := NewConn("tcp", "vk", f.addr, common.WORK_MAIN, "")
	if err == nil || !strings.Contains(err.Error(), "ALERT") {
		t.Fatalf("an unproven server must be refused with an alert, got %v", err)
	}
}

// The client sends its hello and its proof and nothing more until the server
// has proved itself: no application data, and in particular not the key of a
// secret tunnel.
func TestNoApplicationDataIsSentBeforeTheServerProves(t *testing.T) {
	cfg, _ := bridgetls.ServerConfig()
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	got := make(chan int, 1)
	go func() {
		raw, _ := l.Accept()
		c, err := bridgetls.Server(raw, cfg)
		if err != nil {
			return
		}
		defer c.Close()
		// never answers; counts what arrives
		hello := bridgetls.Hello(version.Protocol, version.VERSION, common.WORK_SECRET)
		buf := make([]byte, 4096)
		n, _ := c.Read(buf)
		got <- n - len(hello) - bridgetls.ProofLen
	}()
	go NewConn("tcp", "vk", l.Addr().String(), common.WORK_SECRET, "")
	if extra := <-got; extra != 0 {
		t.Fatalf("%d bytes beyond the hello and its proof were sent before the server proved itself", extra)
	}
}
