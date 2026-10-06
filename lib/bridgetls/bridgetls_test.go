package bridgetls

import (
	"crypto/tls"
	"io"
	"net"
	"strings"
	"testing"
)

// pair joins a client and a server over loopback TCP, each TLS.
func pair(t *testing.T, relay func(client, server net.Conn)) (c, s *tls.Conn) {
	cfg, err := ServerConfig()
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	sc := make(chan *tls.Conn, 1)
	go func() {
		raw, err := l.Accept()
		if err != nil {
			sc <- nil
			return
		}
		x, err := Server(raw, cfg)
		if err != nil {
			sc <- nil
			return
		}
		sc <- x
	}()
	raw, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c, err = Client(raw, ClientConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s = <-sc; s == nil {
		t.Fatal("server side failed")
	}
	t.Cleanup(func() { c.Close(); s.Close() })
	return
}

func TestHandshakeIsTLS13AndBothEndsDeriveTheSameExporter(t *testing.T) {
	c, s := pair(t, nil)
	if v := c.ConnectionState().Version; v != tls.VersionTLS13 {
		t.Fatalf("negotiated %#x, want TLS 1.3", v)
	}
	a, err1 := Exporter(c)
	b, err2 := Exporter(s)
	if err1 != nil || err2 != nil || len(a) != 32 || string(a) != string(b) {
		t.Fatalf("exporters differ or fail: %x %x %v %v", a, b, err1, err2)
	}
	// another session must derive another one
	c2, _ := pair(t, nil)
	if x, _ := Exporter(c2); string(x) == string(a) {
		t.Fatal("two sessions share an exporter")
	}
}

func TestProofs(t *testing.T) {
	c, s := pair(t, nil)
	ec, _ := Exporter(c)
	es, _ := Exporter(s)
	hello := []byte("2 v0.27 main")

	cp := ClientProof("secret-vkey", ec, hello)
	if !Equal(cp, ClientProof("secret-vkey", es, hello)) {
		t.Fatal("the server cannot verify an honest client")
	}
	if Equal(cp, ClientProof("another-vkey", es, hello)) {
		t.Fatal("a wrong vkey verified")
	}
	if Equal(cp, ClientProof("secret-vkey", es, []byte("2 v0.27 chan"))) {
		t.Fatal("a changed hello verified")
	}
	sp := ServerProof("secret-vkey", es, cp)
	if !Equal(sp, ServerProof("secret-vkey", ec, cp)) {
		t.Fatal("the client cannot verify an honest server")
	}
	if Equal(sp, ServerProof("secret-vkey", es, ClientProof("secret-vkey", es, []byte("other")))) {
		t.Fatal("the server proof is not tied to the client's")
	}
	if Equal(cp, sp) {
		t.Fatal("the two directions are not told apart")
	}
}

// The attack that a static secret cannot stop: a man in the middle terminates
// TLS from the client, opens his own TLS to the server, and carries every
// application byte across unchanged. The proof the client makes is for the
// first session, and the server checks it against the second.
func TestRelayingMitmCannotCarryTheProofAcross(t *testing.T) {
	srvCfg, _ := ServerConfig()
	srvL, _ := net.Listen("tcp", "127.0.0.1:0")
	defer srvL.Close()
	mitmCfg, _ := ServerConfig()
	mitmL, _ := net.Listen("tcp", "127.0.0.1:0")
	defer mitmL.Close()

	serverSaw := make(chan []byte, 1)
	serverExp := make(chan []byte, 1)
	go func() { // the real server: reads a proof, reports it and its own exporter
		raw, _ := srvL.Accept()
		s, err := Server(raw, srvCfg)
		if err != nil {
			return
		}
		defer s.Close()
		e, _ := Exporter(s)
		serverExp <- e
		p := make([]byte, ProofLen)
		io.ReadFull(s, p)
		serverSaw <- p
	}()
	go func() { // the man in the middle
		raw, _ := mitmL.Accept()
		down, err := Server(raw, mitmCfg)
		if err != nil {
			return
		}
		up0, _ := net.Dial("tcp", srvL.Addr().String())
		up, err := Client(up0, ClientConfig())
		if err != nil {
			return
		}
		go io.Copy(up, down)
		io.Copy(down, up)
	}()

	raw, _ := net.Dial("tcp", mitmL.Addr().String())
	c, err := Client(raw, ClientConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ec, _ := Exporter(c)
	hello := []byte("hello")
	if _, err := c.Write(ClientProof("vkey", ec, hello)); err != nil {
		t.Fatal(err)
	}
	got := <-serverSaw
	es := <-serverExp
	if string(ec) == string(es) {
		t.Fatal("the two legs share an exporter, the relay would work")
	}
	if !Equal(got, ClientProof("vkey", ec, hello)) {
		t.Fatal("the relay changed the bytes, the test proves nothing")
	}
	if Equal(got, ClientProof("vkey", es, hello)) {
		t.Fatal("the proof carried across the relay verified at the server")
	}
}

func TestOlderTLSIsRefused(t *testing.T) {
	cfg, _ := ServerConfig()
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	go func() {
		raw, err := l.Accept()
		if err == nil {
			Server(raw, cfg)
		}
	}()
	cc := ClientConfig()
	cc.MinVersion, cc.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
	raw, _ := net.Dial("tcp", l.Addr().String())
	if c, err := Client(raw, cc); err == nil {
		c.Close()
		t.Fatal("a TLS 1.2 client got through")
	}
}

func TestPlainClientIsRefused(t *testing.T) {
	cfg, _ := ServerConfig()
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	go func() {
		raw, err := l.Accept()
		if err == nil {
			Server(raw, cfg)
		}
	}()
	raw, _ := net.Dial("tcp", l.Addr().String())
	defer raw.Close()
	raw.Write([]byte("TST plain old handshake"))
	raw.(*net.TCPConn).CloseWrite()
	b, _ := io.ReadAll(raw)
	if strings.Contains(string(b), "sucs") || len(b) > 7 && string(b[:3]) != "\x15\x03\x03" {
		t.Fatalf("a plaintext client got an answer other than a TLS alert: %q", b)
	}
}

func TestCertificateHasNoFixedName(t *testing.T) {
	a, _ := ServerConfig()
	b, _ := ServerConfig()
	if string(a.Certificates[0].Certificate[0]) == string(b.Certificates[0].Certificate[0]) {
		t.Fatal("two servers present the same certificate")
	}
}
