package nps_mux

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func statsRec(b byte) []byte {
	r := make([]byte, StatsLen)
	r[0] = StatsMagic
	r[1] = b
	return r
}

// A record on a client's ping reaches the server's sink, and the echo does
// not leave the client with a timestamp it cannot parse: its latency is still
// measured.
func TestStatsRideOnPing(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 4)
	srvCh := make(chan *Mux, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		srvCh <- NewMuxStats(c, "tcp", 0, nil, func(b []byte) { got <- b })
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cli := NewMuxStats(c, "tcp", 0, func() []byte { return statsRec(7) }, nil)
	srv := <-srvCh
	defer cli.Close()
	defer srv.Close()
	select {
	case b := <-got:
		if len(b) != StatsLen || b[0] != StatsMagic || b[1] != 7 {
			t.Fatalf("record %v", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no record")
	}
	for i := 0; i < 100 && atomic.LoadUint64(&cli.latency) == 0; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if atomic.LoadUint64(&cli.latency) == 0 {
		t.Fatal("the echo with the record did not give a latency")
	}
}

// A client keeps its record to itself until the server says it takes stats:
// against a server that never does, nothing reaches the sink and the pings
// still work.
func TestStatsNotSentToServerThatDoesNotTakeThem(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	seen := make(chan []byte, 8)
	srvCh := make(chan *Mux, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		// no sink, as an older server: it must see no record, only echo
		srvCh <- NewMux(c, "tcp", 0)
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	asked := make(chan struct{}, 8)
	cli := NewMuxStats(c, "tcp", 0, func() []byte { asked <- struct{}{}; return statsRec(7) }, nil)
	srv := <-srvCh
	defer cli.Close()
	defer srv.Close()
	for i := 0; i < 100 && atomic.LoadUint64(&cli.latency) == 0; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if atomic.LoadUint64(&cli.latency) == 0 {
		t.Fatal("no latency")
	}
	select {
	case <-asked:
		t.Fatal("client read its stats for a server that did not ask")
	case <-seen:
	default:
	}
	if atomic.LoadUint32(&cli.statsAccepted) != 0 {
		t.Fatal("accepted without being told")
	}
}

func TestStripStats(t *testing.T) {
	ts := []byte("2026-10-03T00:00:00Z")
	if got := stripStats(ts); string(got) != string(ts) {
		t.Fatal("changed a bare timestamp")
	}
	if got := stripStats(append(statsRec(1), ts...)); string(got) != string(ts) {
		t.Fatalf("got %q", got)
	}
	if got := stripStats(append([]byte{StatsAccept}, ts...)); string(got) != string(ts) {
		t.Fatalf("got %q", got)
	}
}
