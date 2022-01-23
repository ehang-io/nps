package rate

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Rate is an implementation of the token bucket added regularly
type Rate struct {
	bucketSize        int64
	bucketSurplusSize int64
	bucketAddSize     int64
	stopChan          chan bool
	nowRate           int64
	cond              *sync.Cond
	hasStop           bool
	hasStart          bool
}

// NewRate return token bucket with specified rate
func NewRate(addSize int64) *Rate {
	r := &Rate{
		bucketSize:        addSize * 2,
		bucketSurplusSize: 0,
		bucketAddSize:     addSize,
		stopChan:          make(chan bool),
		cond:              sync.NewCond(new(sync.Mutex)),
	}
	return r
}

// Start is used to add token regularly
func (r *Rate) Start() {
	if !r.hasStart {
		r.hasStart = true
		go r.session()
	}
}

func (r *Rate) add(size int64) {
	if res := r.bucketSize - r.bucketSurplusSize; res < r.bucketAddSize {
		atomic.AddInt64(&r.bucketSurplusSize, res)
		return
	}
	atomic.AddInt64(&r.bucketSurplusSize, size)
}

// Write is called when add token to bucket
func (r *Rate) Write(size int64) {
	r.add(size)
}

// Stop is called when not use the rate bucket
func (r *Rate) Stop() {
	if r.hasStart {
		r.stopChan <- true
		r.hasStop = true
		r.cond.Broadcast()
	}
}

// Get is called when get token from bucket
func (r *Rate) Get(size int64) error {
	if r.hasStop {
		return errors.New("the rate has closed")
	}
	if r.bucketSurplusSize >= size {
		atomic.AddInt64(&r.bucketSurplusSize, -size)
		return nil
	}
	for {
		r.cond.L.Lock()
		r.cond.Wait()
		if r.bucketSurplusSize >= size {
			r.cond.L.Unlock()
			atomic.AddInt64(&r.bucketSurplusSize, -size)
			return nil
		}
		if r.hasStop {
			return errors.New("the rate has closed")
		}
		r.cond.L.Unlock()
	}
}

// GetNowRate returns the current rate
// Just a rough number
func (r *Rate) GetNowRate() int64 {
	return r.nowRate
}

func (r *Rate) session() {
	ticker := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-ticker.C:
			if rs := r.bucketAddSize - r.bucketSurplusSize; rs > 0 {
				r.nowRate = rs
			} else {
				r.nowRate = r.bucketSize - r.bucketSurplusSize
			}
			r.add(r.bucketAddSize)
			r.cond.Broadcast()
		case <-r.stopChan:
			ticker.Stop()
			return
		}
	}
}
