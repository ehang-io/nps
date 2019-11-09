package common

import (
	"bytes"
	"sync"
)

const PoolSize = 64 * 1024
const PoolSizeSmall = 100
const PoolSizeUdp = 1472
const PoolSizeCopy = 32 << 10
const PoolSizeBuffer = 4096
const PoolSizeWindow = PoolSizeBuffer - 16 - 32 - 32 - 8

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
			return make([]byte, PoolSizeWindow, PoolSizeWindow)
		},
	}
}

func (Self *windowBufferPool) Get() (buf []byte) {
	buf = Self.pool.Get().([]byte)
	return buf[:PoolSizeWindow]
}

func (Self *windowBufferPool) Put(x []byte) {
	Self.pool.Put(x[:PoolSizeWindow]) // make buf to full
}

type bufferPool struct {
	pool sync.Pool
}

func (Self *bufferPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, PoolSizeBuffer))
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
	return Self.pool.Get().(*MuxPackager)
}

func (Self *muxPackagerPool) Put(pack *MuxPackager) {
	Self.pool.Put(pack)
}

type ListElement struct {
	Buf  []byte
	L    uint16
	Part bool
}

type listElementPool struct {
	pool sync.Pool
}

func (Self *listElementPool) New() {
	Self.pool = sync.Pool{
		New: func() interface{} {
			element := ListElement{}
			return &element
		},
	}
}

func (Self *listElementPool) Get() *ListElement {
	return Self.pool.Get().(*ListElement)
}

func (Self *listElementPool) Put(element *ListElement) {
	element.L = 0
	element.Buf = nil
	element.Part = false
	Self.pool.Put(element)
}

var once = sync.Once{}
var BuffPool = bufferPool{}
var CopyBuff = copyBufferPool{}
var MuxPack = muxPackagerPool{}
var WindowBuff = windowBufferPool{}
var ListElementPool = listElementPool{}

func newPool() {
	BuffPool.New()
	CopyBuff.New()
	MuxPack.New()
	WindowBuff.New()
	ListElementPool.New()
}

func init() {
	once.Do(newPool)
}
