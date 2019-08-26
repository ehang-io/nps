package pool

import (
	"bytes"
	"sync"
)

const PoolSize = 64 * 1024
const PoolSizeSmall = 100
const PoolSizeUdp = 1472
const PoolSizeCopy = 32 << 10

var BufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, PoolSize)
	},
}

var BufPoolUdp = sync.Pool{
	New: func() interface{} {
		return make([]byte, PoolSizeUdp)
	},
}
var BufPoolMax = sync.Pool{
	New: func() interface{} {
		return make([]byte, PoolSize)
	},
}
var BufPoolSmall = sync.Pool{
	New: func() interface{} {
		return make([]byte, PoolSizeSmall)
	},
}
var BufPoolCopy = sync.Pool{
	New: func() interface{} {
		return make([]byte, PoolSizeCopy)
	},
}

func PutBufPoolUdp(buf []byte) {
	if cap(buf) == PoolSizeUdp {
		BufPoolUdp.Put(buf[:PoolSizeUdp])
	}
}

func PutBufPoolCopy(buf []byte) {
	if cap(buf) == PoolSizeCopy {
		BufPoolCopy.Put(buf[:PoolSizeCopy])
	}
}

func GetBufPoolCopy() []byte {
	return (BufPoolCopy.Get().([]byte))[:PoolSizeCopy]
}

func PutBufPoolMax(buf []byte) {
	if cap(buf) == PoolSize {
		BufPoolMax.Put(buf[:PoolSize])
	}
}

type BufferPool struct {
	pool sync.Pool
}

func (Self *BufferPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
}

func (Self *BufferPool) Get() *bytes.Buffer {
	return Self.pool.Get().(*bytes.Buffer)
}

func (Self *BufferPool) Put(x *bytes.Buffer) {
	x.Reset()
	Self.pool.Put(x)
}

var once = sync.Once{}
var BuffPool = BufferPool{}

func init() {
	once.Do(BuffPool.New)
}
