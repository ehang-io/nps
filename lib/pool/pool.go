package pool

import "sync"

type BufferPool struct {
	pool     sync.Pool
	poolSize int
}

func NewBufferPool(poolSize int) *BufferPool {
	bp := &BufferPool{}
	bp.pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, poolSize, poolSize)
		},
	}
	bp.poolSize = poolSize
	return bp
}

func (bp *BufferPool) Get() []byte {
	buf := bp.pool.Get().([]byte)
	return buf[:bp.poolSize] // just like make a new slice, but data may not be 0
}

func (bp *BufferPool) Put(x []byte) {
	if len(x) == bp.poolSize {
		bp.pool.Put(x)
	} else {
		x = nil // buf is not full, not allowed, New method returns a full buf
	}
}
