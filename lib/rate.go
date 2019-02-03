package lib

import (
	"sync/atomic"
	"time"
)

type Rate struct {
	bucketSize        int64     //木桶容量
	bucketSurplusSize int64     //当前桶中体积
	bucketAddSize     int64     //每次加水大小
	stopChan          chan bool //停止
}

func NewRate(addSize int64) *Rate {
	return &Rate{
		bucketSize:        addSize * 2,
		bucketSurplusSize: 0,
		bucketAddSize:     addSize,
		stopChan:          make(chan bool),
	}
}

func (s *Rate) Start() {
	go s.session()
}

func (s *Rate) add(size int64) {
	if (s.bucketSize - s.bucketSurplusSize) < s.bucketAddSize {
		return
	}
	atomic.AddInt64(&s.bucketSurplusSize, size)
}

//回桶
func (s *Rate) ReturnBucket(size int64) {
	s.add(size)
}

//停止
func (s *Rate) Stop() {
	s.stopChan <- true
}

func (s *Rate) Get(size int64) {
	if s.bucketSurplusSize >= size {
		atomic.AddInt64(&s.bucketSurplusSize, -size)
		return
	}
	ticker := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-ticker.C:
			if s.bucketSurplusSize >= size {
				atomic.AddInt64(&s.bucketSurplusSize, -size)
				ticker.Stop()
				return
			}
		}
	}
}

func (s *Rate) session() {
	ticker := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-ticker.C:
			s.add(s.bucketAddSize)
		case <-s.stopChan:
			ticker.Stop()
			return
		}
	}
}
