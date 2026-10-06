package nps_mux

import (
	"io"
	"net"
	"testing"
	"time"
)

// Closing a mux must close its socket. It used to keep a dup of the fd, taken
// with File() and never closed, so the peer saw no FIN and the socket sat in
// CLOSE-WAIT until a finaliser ran: one leaked socket per lost session.
func TestCloseReleasesSocket(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			close(accepted)
			return
		}
		accepted <- c
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	peer := <-accepted
	if peer == nil {
		t.Fatal("accept failed")
	}
	defer peer.Close()

	m := NewMux(c, "tcp", 60)
	time.Sleep(100 * time.Millisecond)
	_ = m.Close()

	// Drain whatever the mux sent (its first ping) and wait for the FIN.
	_ = peer.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.Copy(io.Discard, peer); err != nil {
		t.Fatalf("peer did not see the mux close its socket: %v", err)
	}
}
