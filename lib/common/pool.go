package common

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

type CopyBufferPool struct {
	pool sync.Pool
}

func (Self *CopyBufferPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, PoolSizeCopy, PoolSizeCopy)
		},
	}
}

func (Self *CopyBufferPool) Get() []byte {
	buf := Self.pool.Get().([]byte)
	return buf[:PoolSizeCopy] // just like make a new slice, but data may not be 0
}

func (Self *CopyBufferPool) Put(x []byte) {
	if len(x) == PoolSizeCopy {
		Self.pool.Put(x)
	} else {
		x = nil // buf is not full, maybe truncated by gc in pool, not allowed
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

type MuxPackagerPool struct {
	pool sync.Pool
}

func (Self *MuxPackagerPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			pack := MuxPackager{}
			return &pack
		},
	}
}

func (Self *MuxPackagerPool) Get() *MuxPackager {
	pack := Self.pool.Get().(*MuxPackager)
	buf := CopyBuff.Get()
	pack.Content = buf
	return pack
}

func (Self *MuxPackagerPool) Put(pack *MuxPackager) {
	CopyBuff.Put(pack.Content)
	Self.pool.Put(pack)
}

var once = sync.Once{}
var BuffPool = BufferPool{}
var CopyBuff = CopyBufferPool{}
var MuxPack = MuxPackagerPool{}

func newPool() {
	BuffPool.New()
	CopyBuff.New()
	MuxPack.New()
}

func init() {
	once.Do(newPool)
}
