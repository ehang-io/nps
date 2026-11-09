package proxy

import (
	"bufio"
	"encoding/base64"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"ehang.io/nps/lib/file"
)

func encode(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

func TestCheckBasicCredential(t *testing.T) {
	good := encode("user", "p&#ss:word")
	if !checkBasicCredential(good, "user", "p&#ss:word") {
		t.Fatal("correct credential rejected")
	}
	if checkBasicCredential(good, "user", "other") {
		t.Fatal("wrong password accepted")
	}
	if checkBasicCredential(good, "other", "p&#ss:word") {
		t.Fatal("wrong user accepted")
	}
	if checkBasicCredential("not-base64!!", "user", "p") {
		t.Fatal("invalid base64 accepted")
	}
	if checkBasicCredential(base64.StdEncoding.EncodeToString([]byte("nocolon")), "nocolon", "") {
		t.Fatal("credential without separator accepted")
	}
}

func TestParseAuthLine(t *testing.T) {
	cred := encode("u", "p")
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"AUTH Basic " + cred, cred, true},
		{"auth basic " + cred, cred, true},
		{"Authorization: Basic " + cred, cred, true},
		{"Proxy-Authorization: basic " + cred, cred, true},
		{"GET / HTTP/1.1", "", false},
		{"AUTH " + cred, "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := parseAuthLine(c.line)
		if ok != c.ok || got != c.want {
			t.Fatalf("parseAuthLine(%q) = (%q,%v), want (%q,%v)", c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestFindBasicCredentialAndStrip(t *testing.T) {
	cred := encode("u", "p")
	header := []string{"Host: target", "Authorization: Basic " + cred, "X-Test: 1"}
	if got := findBasicCredential(header); got != cred {
		t.Fatalf("findBasicCredential = %q, want %q", got, cred)
	}
	replay := string(stripAuthHeader("GET / HTTP/1.1", header))
	if strings.Contains(strings.ToLower(replay), "authorization") {
		t.Fatalf("auth header leaked to the target: %q", replay)
	}
	if !strings.Contains(replay, "X-Test: 1") {
		t.Fatalf("other headers must be kept: %q", replay)
	}
	if !strings.HasPrefix(replay, "GET / HTTP/1.1\r\n") || !strings.HasSuffix(replay, "\r\n\r\n") {
		t.Fatalf("replayed request is malformed: %q", replay)
	}
}

func TestIsHttpRequestLine(t *testing.T) {
	if !isHttpRequestLine("CONNECT 1.2.3.4:3389 HTTP/1.1") {
		t.Fatal("CONNECT should be recognised")
	}
	if !isHttpRequestLine("GET / HTTP/1.0") {
		t.Fatal("GET should be recognised")
	}
	if isHttpRequestLine("AUTH Basic abc") {
		t.Fatal("AUTH line must not be treated as HTTP")
	}
	if isHttpRequestLine("hello world") {
		t.Fatal("plain text must not be treated as HTTP")
	}
}

type wrapResult struct {
	conn net.Conn
	err  error
}

func TestWrapTunnelConnAuthOK(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	task := &file.Tunnel{Mode: "tcp", AuthUser: "u", AuthPass: "p"}

	done := make(chan wrapResult, 1)
	go func() {
		c, err := wrapTunnelConn(srv, task)
		done <- wrapResult{c, err}
	}()

	if _, err := cli.Write([]byte("AUTH Basic " + encode("u", "p") + "\r\nafter-auth")); err != nil {
		t.Fatal(err)
	}
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	ack := make([]byte, len(authOKBytes))
	if _, err := io.ReadFull(cli, ack); err != nil {
		t.Fatalf("read auth ack: %v", err)
	}
	if string(ack) != authOKBytes {
		t.Fatalf("want %q, got %q", authOKBytes, ack)
	}

	r := <-done
	if r.err != nil {
		t.Fatalf("auth should succeed: %v", r.err)
	}
	_ = r.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	replay := make([]byte, len("after-auth"))
	if _, err := io.ReadFull(r.conn, replay); err != nil {
		t.Fatalf("read replayed bytes: %v", err)
	}
	if string(replay) != "after-auth" {
		t.Fatalf("replayed bytes = %q", replay)
	}
}

func TestWrapTunnelConnRejectsWrongPassword(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	task := &file.Tunnel{Mode: "tcp", AuthUser: "u", AuthPass: "p"}
	done := make(chan wrapResult, 1)
	go func() {
		c, err := wrapTunnelConn(srv, task)
		done <- wrapResult{c, err}
	}()
	if _, err := cli.Write([]byte("AUTH Basic " + encode("u", "bad") + "\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	ack := make([]byte, len(authFailedBytes))
	if _, err := io.ReadFull(cli, ack); err != nil {
		t.Fatalf("read reject ack: %v", err)
	}
	r := <-done
	if r.err == nil {
		t.Fatal("wrong password must be rejected")
	}
}

func TestWrapTunnelConnHttpConnect(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	task := &file.Tunnel{Mode: "tcp", AuthUser: "u", AuthPass: "p"}
	done := make(chan wrapResult, 1)
	go func() {
		c, err := wrapTunnelConn(srv, task)
		done <- wrapResult{c, err}
	}()
	req := "CONNECT 127.0.0.1:3389 HTTP/1.1\r\nHost: 127.0.0.1:3389\r\nProxy-Authorization: Basic " + encode("u", "p") + "\r\n\r\n"
	if _, err := cli.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(cli)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read connect response: %v", err)
	}
	if !strings.Contains(line, "200 Connection established") {
		t.Fatalf("want 200 connection established, got %q", line)
	}
	r := <-done
	if r.err != nil {
		t.Fatalf("CONNECT auth should succeed: %v", r.err)
	}
	_ = r.conn.Close()
}

func TestWrapTunnelConnSkipsNonTcpMode(t *testing.T) {
	cli, srv := net.Pipe()
	defer cli.Close()
	defer srv.Close()
	task := &file.Tunnel{Mode: "socks5", AuthUser: "u", AuthPass: "p"}
	got, err := wrapTunnelConn(srv, task)
	if err != nil {
		t.Fatal(err)
	}
	if got != srv {
		t.Fatal("non tcp tunnels must not be wrapped")
	}
	if _, err := wrapTunnelConn(srv, nil); err != nil {
		t.Fatal(err)
	}
}
