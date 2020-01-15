package common

import (
	"sync"
)

const PoolSize = 64 * 1024
const PoolSizeSmall = 100
const PoolSizeUdp = 1472 + 200
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

type copyBufferPool struct {
	pool sync.Pool
}

func (Self *copyBufferPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, PoolSizeCopy, PoolSizeCopy)
		},
	}
}

func (Self *copyBufferPool) Get() []byte {
	buf := Self.pool.Get().([]byte)
	return buf[:PoolSizeCopy] // just like make a new slice, but data may not be 0
}

func (Self *copyBufferPool) Put(x []byte) {
	if len(x) == PoolSizeCopy {
		Self.pool.Put(x)
	} else {
		x = nil // buf is not full, not allowed, New method returns a full buf
	}
}

var once = sync.Once{}
var CopyBuff = copyBufferPool{}

func newPool() {
	CopyBuff.New()
}

func init() {
	once.Do(newPool)
}
