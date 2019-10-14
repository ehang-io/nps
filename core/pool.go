package core

import (
	"bytes"
	"sync"
)

const PoolSize = 64 * 1024
const PoolSizeSmall = 100
const PoolSizeUdp = 1472
const PoolSizeCopy = 32 << 10
const PoolSizeWindow = 1<<16 - 1

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

type windowBufferPool struct {
	pool sync.Pool
}

func (Self *windowBufferPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, PoolSizeWindow)
		},
	}
}

func (Self *windowBufferPool) Get() (buf []byte) {
	buf = Self.pool.Get().([]byte)
	return buf[:0]
}

func (Self *windowBufferPool) Put(x []byte) {
	if cap(x) == PoolSizeWindow {
		Self.pool.Put(x[:PoolSizeWindow]) // make buf to full
	} else {
		x = nil
	}
}

type bufferPool struct {
	pool sync.Pool
}

func (Self *bufferPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
}

func (Self *bufferPool) Get() *bytes.Buffer {
	return Self.pool.Get().(*bytes.Buffer)
}

func (Self *bufferPool) Put(x *bytes.Buffer) {
	x.Reset()
	Self.pool.Put(x)
}

type muxPackagerPool struct {
	pool sync.Pool
}

func (Self *muxPackagerPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			pack := MuxPackager{}
			return &pack
		},
	}
}

func (Self *muxPackagerPool) Get() *MuxPackager {
	pack := Self.pool.Get().(*MuxPackager)
	buf := CopyBuff.Get()
	pack.Content = buf
	return pack
}

func (Self *muxPackagerPool) Put(pack *MuxPackager) {
	CopyBuff.Put(pack.Content)
	Self.pool.Put(pack)
}

var once = sync.Once{}
var BuffPool = bufferPool{}
var CopyBuff = copyBufferPool{}
var MuxPack = muxPackagerPool{}
var WindowBuff = windowBufferPool{}

func newPool() {
	BuffPool.New()
	CopyBuff.New()
	MuxPack.New()
	WindowBuff.New()
}

func init() {
	once.Do(newPool)
}
