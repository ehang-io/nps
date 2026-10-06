package sysstat

import (
	"sync"
	"time"

	nps_mux "ehang.io/nps-mux"
)

// Retention is how long History keeps a record.
const Retention = 2 * time.Hour

// historyCap holds Retention of records at the 5s ping interval, with room
// for a client that reconnects and pings a little more often.
const historyCap = int(Retention/(5*time.Second)) + 60

// Point is one record and when it arrived.
type Point struct {
	At  time.Time
	Rec [nps_mux.StatsLen]byte
}

// History is a ring of a client's most recent records. The zero value is
// ready to use.
type History struct {
	mu   sync.Mutex
	ring []Point
	next int // where the next record goes
	n    int
}

// Add stores a record received now. Anything of the wrong length is dropped.
func (h *History) Add(at time.Time, rec []byte) {
	if len(rec) != nps_mux.StatsLen {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ring == nil {
		h.ring = make([]Point, historyCap)
	}
	p := &h.ring[h.next]
	p.At = at
	copy(p.Rec[:], rec)
	h.next = (h.next + 1) % len(h.ring)
	if h.n < len(h.ring) {
		h.n++
	}
}

// Points returns the records no older than Retention before now, oldest
// first.
func (h *History) Points(now time.Time) []Point {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Point, 0, h.n)
	for i := 0; i < h.n; i++ {
		p := h.ring[(h.next-h.n+i+len(h.ring))%len(h.ring)]
		if now.Sub(p.At) <= Retention {
			out = append(out, p)
		}
	}
	return out
}

// Last is the newest record, if any.
func (h *History) Last() (Point, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n == 0 {
		return Point{}, false
	}
	return h.ring[(h.next-1+len(h.ring))%len(h.ring)], true
}
