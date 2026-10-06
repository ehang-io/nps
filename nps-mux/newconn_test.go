package nps_mux

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// NewConn used to wait out its full two minute timer when the mux closed
// underneath it: Close wakes every conn in the map, but nothing in NewConn
// was listening.
func TestNewConnReturnsWhenMuxCloses(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe() // b never answers
	defer b.Close()
	m := NewMux(a, "tcp", 60)
	done := make(chan error, 1)
	go func() {
		_, err := m.NewConn()
		done <- err
	}()
	time.Sleep(100 * time.Millisecond) // let it send and start waiting
	_ = m.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("NewConn on a closed mux returned no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("NewConn kept waiting after its mux closed")
	}
}

// A reply to a NewConn that is no longer waiting, such as the OK that comes
// after the two minute timer, used to block the mux's read loop on an
// unbuffered send, stalling every stream on the mux.
func TestUnclaimedNewConnReplyDoesNotStallReadLoop(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe()
	client := NewMux(a, "tcp", 60)
	server := NewMux(b, "tcp", 60)
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	go func() {
		for {
			if _, err := server.Accept(); err != nil {
				return
			}
		}
	}()
	c, err := client.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	// the OK for c was consumed; these have nobody to receive them
	server.sendInfo(muxNewConnOk, c.connId, nil)
	server.sendInfo(muxNewConnFail, c.connId, nil)
	server.sendInfo(muxNewConnOk, c.connId, nil)

	done := make(chan error, 1)
	go func() {
		_, err := client.NewConn()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the read loop is stuck behind an unclaimed reply")
	}
}

// A stream opened without waiting for the OK sends its data right behind the
// opening frame. The peer may be a long way from accepting it, and the data
// must wait for it, not be dropped for want of a stream to put it in.
func TestNoWaitDataSurvivesASlowAccept(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe()
	client := NewMux(a, "tcp", 60)
	server := NewMux(b, "tcp", 60)
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })

	got := make(chan string, 1)
	go func() {
		time.Sleep(400 * time.Millisecond) // nobody accepts for a while
		c, err := server.Accept()
		if err != nil {
			got <- err.Error()
			return
		}
		defer c.Close()
		buf := make([]byte, 5)
		if _, err := io.ReadFull(c, buf); err != nil {
			got <- err.Error()
			return
		}
		c.Write([]byte("world"))
		got <- string(buf)
	}()

	c, err := client.NewConnNoWait()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 5)
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(c, reply); err != nil || string(reply) != "world" {
		t.Fatalf("reply %q, %v", reply, err)
	}
	if s := <-got; s != "hello" {
		t.Fatalf("the server read %q", s)
	}
}

// Many streams opened and written to at once, none waited for.
func TestNoWaitManyStreams(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe()
	client := NewMux(a, "tcp", 60)
	server := NewMux(b, "tcp", 60)
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	go func() {
		for {
			c, err := server.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				buf := make([]byte, 64)
				n, err := io.ReadFull(c, buf)
				if err == nil {
					c.Write(buf[:n])
				}
			}()
		}
	}()
	const n = 200
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			c, err := client.NewConnNoWait()
			if err != nil {
				errs <- err
				return
			}
			defer c.Close()
			msg := make([]byte, 64)
			msg[0] = byte(i)
			c.Write(msg)
			c.SetReadDeadline(time.Now().Add(10 * time.Second))
			back := make([]byte, 64)
			if _, err := io.ReadFull(c, back); err != nil || back[0] != byte(i) {
				errs <- err
				return
			}
			errs <- nil
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

// A peer cannot have more than maxPendingAccept streams waiting to be
// accepted: past that they are refused, not held.
func TestStreamsPastThePendingBoundAreRefused(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe()
	client := NewMux(a, "tcp", 60)
	server := NewMux(b, "tcp", 60) // nobody calls Accept on it
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	var opened []*conn
	for i := 0; i < maxPendingAccept+50; i++ {
		c, err := client.NewConnNoWait()
		if err != nil {
			t.Fatal(err)
		}
		opened = append(opened, c)
	}
	waitUntil := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(&server.pendingAccept) < maxPendingAccept && time.Now().Before(waitUntil) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	if n := atomic.LoadInt32(&server.pendingAccept); n != maxPendingAccept {
		t.Fatalf("%d streams waiting, want the bound %d", n, maxPendingAccept)
	}
	// the refused ones are told: their conns are closed from the other side
	closed := 0
	for _, c := range opened[maxPendingAccept:] {
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := c.Read(make([]byte, 1)); err != nil {
			closed++
		}
	}
	if closed != 50 {
		t.Fatalf("%d of the 50 streams past the bound were refused", closed)
	}
}

func TestHealthyFollowsThePings(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe()
	client := NewMuxLiveness(a, "tcp", 5, 100*time.Millisecond)
	server := NewMuxLiveness(b, "tcp", 5, 100*time.Millisecond)
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	time.Sleep(450 * time.Millisecond)
	if !client.Healthy() {
		t.Fatal("a mux whose peer answers is not healthy")
	}
	// a peer that reads and never answers
	c, d := net.Pipe()
	go io.Copy(io.Discard, d)
	silent := NewMuxLiveness(c, "tcp", 5, 100*time.Millisecond)
	t.Cleanup(func() { _ = silent.Close() })
	deadline := time.Now().Add(3 * time.Second)
	for silent.Healthy() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if silent.Healthy() {
		t.Fatal("a mux whose peer never answers stays healthy")
	}
	if silent.IsClose {
		t.Fatal("it should not be closed on this account yet")
	}
}
