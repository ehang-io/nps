package bridge

import (
	"io"
	"net"
	"testing"
	"time"

	"ehang.io/nps-mux"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
)

// A stream of a visitor's secret link opens with the digest of a secret, and
// reaches the secret channel as a connection of its own would: the same
// Secret, and a connection that is the stream.
func TestSecretStreamsReachTheSecretChannel(t *testing.T) {
	b := &Bridge{SecretChan: make(chan *conn.Secret, 4)}
	x, y := net.Pipe()
	visitor := nps_mux.NewMux(x, "tcp", 60)
	server := nps_mux.NewMux(y, "tcp", 60)
	go b.serveSecretStreams(server)

	digest := crypt.Md5("a secret")
	for i := 0; i < 3; i++ {
		st, err := visitor.NewConnNoWait()
		if err != nil {
			t.Fatal(err)
		}
		// the digest and the first bytes of the work, in one flight
		if _, err := st.Write(append([]byte(digest), []byte("and the request")...)); err != nil {
			t.Fatal(err)
		}
		select {
		case s := <-b.SecretChan:
			if s.Password != digest {
				t.Fatalf("the secret is %q, want %q", s.Password, digest)
			}
			rest := make([]byte, len("and the request"))
			s.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.ReadFull(s.Conn, rest); err != nil || string(rest) != "and the request" {
				t.Fatalf("what followed the digest: %q, %v", rest, err)
			}
			// and the stream carries data back
			s.Conn.Write([]byte("ok"))
			back := make([]byte, 2)
			st.SetReadDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.ReadFull(st, back); err != nil || string(back) != "ok" {
				t.Fatalf("reply %q, %v", back, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the stream never reached the secret channel")
		}
	}
}

// One that opens and says nothing is dropped, and does not hold up the ones
// behind it.
func TestASilentStreamIsDroppedAndHoldsUpNoOther(t *testing.T) {
	b := &Bridge{SecretChan: make(chan *conn.Secret, 4)}
	x, y := net.Pipe()
	visitor := nps_mux.NewMux(x, "tcp", 60)
	server := nps_mux.NewMux(y, "tcp", 60)
	go b.serveSecretStreams(server)

	if _, err := visitor.NewConnNoWait(); err != nil { // says nothing
		t.Fatal(err)
	}
	st, _ := visitor.NewConnNoWait()
	digest := crypt.Md5("another")
	st.Write([]byte(digest))
	select {
	case s := <-b.SecretChan:
		if s.Password != digest {
			t.Fatalf("the secret is %q", s.Password)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a silent stream held up the next one")
	}
}
