package client

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps-mux"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"github.com/astaxie/beego/logs"
)

// A visitor used to open a connection to the server for every connection that
// came to its local port, which cost the TCP handshake, the TLS handshake and
// the proof of the vkey before the first byte could go. It now keeps a few
// long-lived links per secret, each a mux that has been through all that once,
// and a new local connection is a new stream on one of them.
//
// The links are an addition, not a replacement. While none is up, which is the
// case at start, after one dies, and always against a server that does not
// know them, a connection is made the old way, so nothing waits for a link and
// nothing stops working without one.

const (
	defaultSecretLinks = 2
	maxSecretLinks     = 8
	// how long a server has to say that it knows the link type; one that does
	// not says nothing at all
	linkReadyWait = 5 * time.Second
	// how long to leave a server alone after it twice did not know the type
	unsupportedRetry = 5 * time.Minute
	// a link that lasted this long was a working one, whatever came after
	linkSteady = 30 * time.Second
	// rates below this are noise: two links are as idle as each other
	idleRate = 16 * 1024 // bytes per second
)

var errNoSecretLinks = errors.New("the server does not offer secret links")

// secretLinkSet is the links of each secret that is being served, by the
// LocalServer that serves it. They are made when the local port is opened and
// stopped when it is closed, and a connection that is still being handled
// after that finds none.
var secretLinkSet sync.Map

type secretLink struct {
	mux    *nps_mux.Mux // nil while the link is down; guarded by secretLinks.mu
	moved  int64        // bytes through the link in either direction; atomic
	active int32        // streams open on the link; atomic
	rate   float64      // moved, smoothed over about a second; guarded by mu
}

type secretLinks struct {
	cfg   *config.CommonConfig
	links []*secretLink

	mu     sync.Mutex
	done   chan struct{}
	closed sync.Once
	everUp int32 // a link has been made to this server at least once; atomic
}

// startSecretLinks makes the links of a secret, or returns nil if it is set to
// have none.
func startSecretLinks(cfg *config.CommonConfig, l *config.LocalServer) *secretLinks {
	if l.Links < 0 {
		return nil
	}
	n := l.Links
	if n == 0 {
		n = defaultSecretLinks
	}
	if n > maxSecretLinks {
		n = maxSecretLinks
	}
	s := &secretLinks{cfg: cfg, done: make(chan struct{})}
	for i := 0; i < n; i++ {
		s.links = append(s.links, new(secretLink))
	}
	if _, loaded := secretLinkSet.LoadOrStore(l, s); loaded {
		return nil
	}
	for i := range s.links {
		go s.maintain(s.links[i])
	}
	go s.sample()
	return s
}

// linksOf is the links of a secret that has them, nil if it has none.
func linksOf(l *config.LocalServer) *secretLinks {
	if v, ok := secretLinkSet.Load(l); ok {
		return v.(*secretLinks)
	}
	return nil
}

// closeSecretLinks stops every secret's links.
func closeSecretLinks() {
	secretLinkSet.Range(func(k, v interface{}) bool {
		v.(*secretLinks).close()
		secretLinkSet.Delete(k)
		return true
	})
}

func (s *secretLinks) close() {
	s.closed.Do(func() {
		close(s.done)
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, l := range s.links {
			if l.mux != nil {
				l.mux.Close()
			}
		}
	})
}

// maintain keeps one link up: connect, hold it until it dies, connect again.
// Trouble is waited out for longer each time, and the wait only starts over
// after a link that has stayed up: a server that takes links and drops them
// at once must not be asked once a second.
func (s *secretLinks) maintain(l *secretLink) {
	backoff := time.Second
	misses := 0
	for {
		select {
		case <-s.done:
			return
		default:
		}
		m, dead, err := s.connect()
		if err != nil {
			wait := backoff
			if errors.Is(err, errNoSecretLinks) {
				// One slow answer proves nothing; the second in a row is an
				// older server, which is then left alone for a while.
				if misses++; misses >= 2 {
					logs.Info("%s; each connection to the local port will have its own", err)
					wait = unsupportedRetry
					misses = 0
				}
			} else {
				misses = 0
				logs.Warn("secret link:", err)
			}
			if backoff *= 2; backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-s.done:
				return
			case <-time.After(wait):
			}
			continue
		}
		misses = 0
		up := time.Now()
		s.mu.Lock()
		l.mux = m
		s.mu.Unlock()
		select {
		case <-dead:
		case <-s.done:
		}
		s.mu.Lock()
		l.mux = nil
		s.mu.Unlock()
		m.Close()
		if time.Since(up) >= linkSteady {
			backoff = time.Second
		}
	}
}

// watchedConn tells when the connection under a mux has closed, which is how
// a link is known to be dead: the mux closes it when its pings go unanswered
// or its reads fail.
type watchedConn struct {
	net.Conn
	once sync.Once
	dead chan struct{}
}

func (w *watchedConn) Close() error {
	w.once.Do(func() { close(w.dead) })
	return w.Conn.Close()
}

// NetConn lets the mux find the socket under this wrapper.
func (w *watchedConn) NetConn() net.Conn { return w.Conn }

// connect makes one link: a connection of the secret-link type, the server's
// word that it knows it, and a mux over it.
func (s *secretLinks) connect() (*nps_mux.Mux, <-chan struct{}, error) {
	c, err := NewConn(s.cfg.Tp, s.cfg.VKey, s.cfg.Server, common.WORK_SECRET_MUX, s.cfg.ProxyUrl)
	if err != nil {
		return nil, nil, err
	}
	c.SetReadDeadlineBySecond(linkReadyWait / time.Second)
	flag, err := c.ReadFlag()
	if err != nil || flag != common.WORK_SECRET_MUX {
		c.Close()
		// Silence, and only silence, is how a server that does not know the
		// type answers. A connection closed on us, or a word that is not the
		// one, is a server that does, and is in trouble.
		var ne net.Error
		if err != nil && errors.As(err, &ne) && ne.Timeout() && atomic.LoadInt32(&s.everUp) == 0 {
			return nil, nil, errNoSecretLinks
		}
		return nil, nil, errors.New("the server did not accept the link")
	}
	atomic.StoreInt32(&s.everUp, 1)
	c.SetAlive()
	w := &watchedConn{Conn: c.Conn, dead: make(chan struct{})}
	return nps_mux.NewMuxLiveness(w, s.cfg.Tp, common.SecretLinkPingCheck, common.SecretLinkPingEvery), w.dead, nil
}

// sample keeps each link's rate current: every 100 ms the bytes moved since
// the last time, as a rate, are mixed into the smoothed one.
func (s *secretLinks) sample() {
	last := make([]int64, len(s.links))
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		s.mu.Lock()
		for i, l := range s.links {
			now := atomic.LoadInt64(&l.moved)
			l.rate = 0.7*l.rate + 0.3*float64(now-last[i])/0.1
			last[i] = now
		}
		s.mu.Unlock()
	}
}

// pick is the link, among those that are up and answering pings, that has
// moved the fewest bytes lately, and among those equally idle the one with the
// fewest streams open, the first of equals after that. nil if none is usable.
// It knows nothing of the stream to come, which is the point: a stream that
// is already running cannot be moved, but a new one can keep away from a link
// that is busy with a large transfer, and a burst of them spreads.
func (s *secretLinks) pick() (*secretLink, *nps_mux.Mux) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best *secretLink
	for _, l := range s.links {
		if l.mux == nil || !l.mux.Healthy() {
			continue
		}
		if best == nil || less(l, best) {
			best = l
		}
	}
	if best == nil {
		return nil, nil
	}
	return best, best.mux
}

// less is whether a should be used before b.
func less(a, b *secretLink) bool {
	ar, br := a.rate, b.rate
	if ar < idleRate {
		ar = 0
	}
	if br < idleRate {
		br = 0
	}
	if ar != br {
		return ar < br
	}
	return atomic.LoadInt32(&a.active) < atomic.LoadInt32(&b.active)
}

// open starts a stream on a link, or reports that there is none to start it
// on. The stream is not waited for: what the visitor writes next goes out
// behind the frame that opens it.
func (s *secretLinks) open() (net.Conn, bool) {
	l, m := s.pick()
	if l == nil {
		return nil, false
	}
	st, err := m.NewConnNoWait()
	if err != nil {
		return nil, false
	}
	atomic.AddInt32(&l.active, 1)
	return &countedConn{Conn: st, link: l}, true
}

// countedConn counts the bytes and the life of a stream into its link.
type countedConn struct {
	net.Conn
	link *secretLink
	once sync.Once
}

func (c *countedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	atomic.AddInt64(&c.link.moved, int64(n))
	return n, err
}

func (c *countedConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	atomic.AddInt64(&c.link.moved, int64(n))
	return n, err
}

func (c *countedConn) Close() error {
	c.once.Do(func() { atomic.AddInt32(&c.link.active, -1) })
	return c.Conn.Close()
}

// the number of links that are up, for tests
func (s *secretLinks) up() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, l := range s.links {
		if l.mux != nil {
			n++
		}
	}
	return n
}
