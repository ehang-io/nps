package client

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ehang.io/nps-mux"
	"ehang.io/nps/lib/bridgetls"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/conn"
)

// Closing a mux trips the race detector on flags upstream never synchronised
// (Mux.IsClose and the id counter, window.closeOp, conn.isClose), the same ones
// nps-mux's own tests skip for. None of them is state of the links.
func skipUnderRace(t *testing.T) {
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
}

// linkFake is a bridge that serves any number of connections: the proofs, then
// by the work the connection says it is for. A secret connection and a secret
// stream both open with the digest of a secret and are echoed.
type linkFake struct {
	addr    string
	ack     bool // whether it knows the secret-link type
	legacy  int32
	streams int32
	linkers int32       // link connections made
	block   atomic.Bool // close link connections instead of serving them

	mu    sync.Mutex
	links []net.Conn
}

func startLinkFake(t *testing.T, vkey string, ack bool) *linkFake {
	cfg, err := bridgetls.ServerConfig()
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	f := &linkFake{addr: l.Addr().String(), ack: ack}
	go func() {
		for {
			raw, err := l.Accept()
			if err != nil {
				return
			}
			go f.serve(raw, cfg, vkey)
		}
	}()
	return f
}

func (f *linkFake) serve(raw net.Conn, cfg *tls.Config, vkey string) {
	c, err := bridgetls.Server(raw, cfg)
	if err != nil {
		return
	}
	sc := conn.NewConn(c)
	protocol, _ := sc.GetShortLenContent()
	vs, _ := sc.GetShortLenContent()
	work, err := sc.ReadFlag()
	if err != nil {
		c.Close()
		return
	}
	proof, err := sc.GetShortContent(bridgetls.ProofLen)
	if err != nil {
		c.Close()
		return
	}
	exporter, _ := bridgetls.Exporter(c)
	hello := bridgetls.Hello(string(protocol), string(vs), work)
	if !bridgetls.Equal(proof, bridgetls.ClientProof(vkey, exporter, hello)) {
		sc.Write([]byte(common.VERIFY_EER))
		c.Close()
		return
	}
	sc.Write(append([]byte(common.VERIFY_SUCCESS), bridgetls.ServerProof(vkey, exporter, proof)...))
	switch work {
	case common.WORK_SECRET:
		defer c.Close()
		if _, err := io.ReadFull(c, make([]byte, 32)); err == nil {
			atomic.AddInt32(&f.legacy, 1)
			io.Copy(c, c)
		}
	case common.WORK_SECRET_MUX:
		atomic.AddInt32(&f.linkers, 1)
		if f.block.Load() {
			c.Close()
			return
		}
		if !f.ack {
			io.Copy(io.Discard, c) // an older server: no word, no mux
			return
		}
		f.mu.Lock()
		f.links = append(f.links, c)
		f.mu.Unlock()
		sc.Write([]byte(common.WORK_SECRET_MUX))
		m := nps_mux.NewMux(c, "tcp", common.SecretLinkPingCheck)
		for {
			st, err := m.Accept()
			if err != nil {
				return
			}
			go func() {
				defer st.Close()
				if _, err := io.ReadFull(st, make([]byte, 32)); err == nil {
					atomic.AddInt32(&f.streams, 1)
					io.Copy(st, st)
				}
			}()
		}
	}
}

func (f *linkFake) killLinks() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.links {
		c.Close()
	}
	f.links = nil
}

func visitorOf(f *linkFake, links int) (*config.CommonConfig, *config.LocalServer) {
	return &config.CommonConfig{Server: f.addr, VKey: "vk-0123456789abcdef", Tp: "tcp"},
		&config.LocalServer{Type: "secret", Password: "pw", Links: links}
}

// through sends a message through handleSecret as a local connection would
// and returns what came back.
func through(t *testing.T, cfg *config.CommonConfig, l *config.LocalServer, msg string) {
	t.Helper()
	a, b := net.Pipe()
	go handleSecret(b, cfg, l)
	defer a.Close()
	a.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := a.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(a, got); err != nil || string(got) != msg {
		t.Fatalf("echo %q, %v", got, err)
	}
}

func waitFor(t *testing.T, what string, d time.Duration, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestSecretLinksCarryTheConnections(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", true)
	cfg, l := visitorOf(f, 2)
	links := startSecretLinks(cfg, l)
	t.Cleanup(closeSecretLinks)
	waitFor(t, "both links", 10*time.Second, func() bool { return links.up() == 2 })

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			through(t, cfg, l, "message-"+string(rune('a'+i)))
		}(i)
	}
	wg.Wait()
	if got := atomic.LoadInt32(&f.streams); got != 20 {
		t.Errorf("%d streams reached the server, want 20", got)
	}
	if got := atomic.LoadInt32(&f.legacy); got != 0 {
		t.Errorf("%d connections of their own were made while links were up", got)
	}
}

// A server that does not know the link type says nothing. Connections must not
// wait for a link that will not come: they go the old way at once.
func TestAnOlderServerGetsConnectionsOfTheirOwn(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", false)
	cfg, l := visitorOf(f, 2)
	startSecretLinks(cfg, l)
	t.Cleanup(closeSecretLinks)

	t0 := time.Now()
	through(t, cfg, l, "first")
	through(t, cfg, l, "second")
	if d := time.Since(t0); d > 3*time.Second {
		t.Fatalf("two connections took %v: they waited for the link", d)
	}
	if got := atomic.LoadInt32(&f.legacy); got != 2 {
		t.Errorf("%d connections of their own, want 2", got)
	}
}

// A link that dies is replaced, and the connections that come meanwhile
// are not lost: while the server will not take links they go the old way.
func TestALinkThatDiesIsReplaced(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", true)
	cfg, l := visitorOf(f, 2)
	links := startSecretLinks(cfg, l)
	t.Cleanup(closeSecretLinks)
	waitFor(t, "both links", 10*time.Second, func() bool { return links.up() == 2 })

	f.block.Store(true) // the server is restarting
	f.killLinks()
	waitFor(t, "the links to be seen dead", 10*time.Second, func() bool { return links.up() == 0 })
	through(t, cfg, l, "while down")
	if atomic.LoadInt32(&f.legacy) != 1 {
		t.Errorf("%d connections of their own while the links were down, want 1", f.legacy)
	}

	f.block.Store(false)
	waitFor(t, "both links again", 20*time.Second, func() bool { return links.up() == 2 })
	through(t, cfg, l, "after")
	if got := atomic.LoadInt32(&f.streams); got != 1 {
		t.Errorf("%d streams after the links came back, want 1", got)
	}
}

// A server that has served links and then is slow to answer one is not one
// that does not know them: the visitor must try again soon, not in five
// minutes.
func TestASlowServerIsNotMistakenForAnOldOne(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", true)
	cfg, l := visitorOf(f, 1)
	links := startSecretLinks(cfg, l)
	t.Cleanup(closeSecretLinks)
	waitFor(t, "the link", 10*time.Second, func() bool { return links.up() == 1 })
	f.block.Store(true)
	f.killLinks()
	waitFor(t, "the link to be seen dead", 10*time.Second, func() bool { return links.up() == 0 })
	time.Sleep(1500 * time.Millisecond) // one refused attempt and its backoff
	f.block.Store(false)
	waitFor(t, "the link again, in seconds", 15*time.Second, func() bool { return links.up() == 1 })
}

func TestLinksOffMeansConnectionsOfTheirOwn(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", true)
	cfg, l := visitorOf(f, -1)
	if startSecretLinks(cfg, l) != nil || linksOf(l) != nil {
		t.Fatal("links were started for a secret that has them off")
	}
	through(t, cfg, l, "plain")
	if atomic.LoadInt32(&f.legacy) != 1 || atomic.LoadInt32(&f.streams) != 0 {
		t.Errorf("legacy %d streams %d", f.legacy, f.streams)
	}
}

// pickable is a link that is up and answers pings, without a connection.
func pickable(t *testing.T, rate float64, active int32) *secretLink {
	a, b := net.Pipe()
	go io.Copy(io.Discard, b)
	m := nps_mux.NewMuxLiveness(a, "tcp", 5, time.Hour) // never pings, so never unhealthy
	t.Cleanup(func() { m.Close() })
	return &secretLink{mux: m, rate: rate, active: active}
}

func TestNewStreamsKeepAwayFromTheBusyLink(t *testing.T) {
	skipUnderRace(t)
	type spec struct {
		rate   float64
		active int32
	}
	for _, c := range []struct {
		name string
		in   []spec
		down []int
		want int
	}{
		{"idle: the first", []spec{{0, 0}, {0, 0}}, nil, 0},
		{"the first is busy", []spec{{500000, 1}, {10, 0}}, nil, 1},
		{"the second is busy", []spec{{10, 0}, {500000, 1}}, nil, 0},
		{"the idler one is down", []spec{{500000, 1}, {10, 0}}, []int{1}, 0},
		{"three, the middle one idlest", []spec{{900000, 1}, {20000, 1}, {400000, 1}}, nil, 1},
		{"none up", []spec{{0, 0}, {0, 0}}, []int{0, 1}, -1},
		// rates under the noise floor are equal, and then the streams open decide
		{"equally idle, fewer streams wins", []spec{{3000, 5}, {10, 1}}, nil, 1},
		{"a burst spreads over the idle links", []spec{{0, 4}, {0, 4}, {0, 3}}, nil, 2},
		{"busy beats stream count", []spec{{900000, 0}, {20000, 9}}, nil, 1},
	} {
		s := &secretLinks{}
		for _, sp := range c.in {
			s.links = append(s.links, pickable(t, sp.rate, sp.active))
		}
		for _, d := range c.down {
			s.links[d].mux.Close()
			s.links[d].mux = nil
		}
		got, _ := s.pick()
		idx := -1
		for i, l := range s.links {
			if l == got {
				idx = i
			}
		}
		if idx != c.want {
			t.Errorf("%s: picked %d, want %d", c.name, idx, c.want)
		}
	}
}

// A link whose peer has stopped answering is not given new streams, though it
// is not known to be dead yet.
func TestALinkThatStoppedAnsweringIsNotPicked(t *testing.T) {
	skipUnderRace(t)
	a, b := net.Pipe()
	go io.Copy(io.Discard, b) // reads, never answers
	silent := nps_mux.NewMuxLiveness(a, "tcp", 50, 50*time.Millisecond)
	t.Cleanup(func() { silent.Close() })
	good := pickable(t, 0, 0)
	s := &secretLinks{links: []*secretLink{{mux: silent}, good}}
	waitFor(t, "the silent link to be unhealthy", 5*time.Second, func() bool { return !silent.Healthy() })
	for i := 0; i < 20; i++ {
		if l, _ := s.pick(); l != good {
			t.Fatal("a link that does not answer pings was picked")
		}
	}
	s.links = s.links[:1]
	if l, _ := s.pick(); l != nil {
		t.Fatal("with nothing healthy, nothing should be picked")
	}
}

// Opening a stream takes the link's count up, closing it takes it down, once.
func TestStreamsAreCountedOnTheirLink(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", true)
	cfg, l := visitorOf(f, 1)
	links := startSecretLinks(cfg, l)
	t.Cleanup(closeSecretLinks)
	waitFor(t, "the link", 10*time.Second, func() bool { return links.up() == 1 })
	st1, ok1 := links.open()
	st2, ok2 := links.open()
	if !ok1 || !ok2 {
		t.Fatal("no stream")
	}
	if got := atomic.LoadInt32(&links.links[0].active); got != 2 {
		t.Fatalf("%d active, want 2", got)
	}
	st1.Close()
	st1.Close() // twice must not count twice
	if got := atomic.LoadInt32(&links.links[0].active); got != 1 {
		t.Fatalf("%d active after one close, want 1", got)
	}
	st2.Close()
	if got := atomic.LoadInt32(&links.links[0].active); got != 0 {
		t.Fatalf("%d active after both closed, want 0", got)
	}
}

// A connection that is still being handled after the links are closed, as one
// is when the control connection drops, finds none and does not start any.
func TestClosedLinksAreNotRestartedByAStragglerOrP2P(t *testing.T) {
	skipUnderRace(t)
	f := startLinkFake(t, "vk-0123456789abcdef", true)
	cfg, l := visitorOf(f, 2)
	links := startSecretLinks(cfg, l)
	waitFor(t, "both links", 10*time.Second, func() bool { return links.up() == 2 })
	closeSecretLinks()
	if linksOf(l) != nil {
		t.Fatal("links remain registered after being closed")
	}
	through(t, cfg, l, "straggler") // the old way, and nothing started
	if linksOf(l) != nil {
		t.Fatal("handling a connection started links again")
	}
	// a secret that was never started, such as a p2p visitor's, has none
	p2p := &config.LocalServer{Type: "p2p", Password: "pw"}
	through(t, cfg, p2p, "p2p fallback")
	if linksOf(p2p) != nil {
		t.Fatal("a p2p visitor's fallback started links")
	}
}
