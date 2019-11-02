package common

import (
	"bytes"
	"github.com/panjf2000/ants/v2"
	"net"
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

type connGroup struct {
	src net.Conn
	dst net.Conn
	wg  *sync.WaitGroup
}

func newConnGroup(src net.Conn, dst net.Conn, wg *sync.WaitGroup) connGroup {
	return connGroup{
		src: src,
		dst: dst,
		wg:  wg,
	}
}

func copyConnGroup(group interface{}) {
	cg, ok := group.(connGroup)
	if !ok {
		return
	}
	_, err := CopyBuffer(cg.src, cg.dst)
	if err != nil {
		cg.src.Close()
		cg.dst.Close()
		//logs.Warn("close npc by copy from nps", err, c.connId)
	}
	cg.wg.Done()
}

type Conns struct {
	conn1 net.Conn
	conn2 net.Conn
}

func NewConns(c1 net.Conn, c2 net.Conn) Conns {
	return Conns{
		conn1: c1,
		conn2: c2,
	}
}

func copyConns(group interface{}) {
	conns := group.(Conns)
	wg := new(sync.WaitGroup)
	wg.Add(2)
	_ = connCopyPool.Invoke(newConnGroup(conns.conn1, conns.conn2, wg))
	_ = connCopyPool.Invoke(newConnGroup(conns.conn2, conns.conn1, wg))
	wg.Wait()
}

var once = sync.Once{}
var BuffPool = bufferPool{}
var CopyBuff = copyBufferPool{}
var MuxPack = muxPackagerPool{}
var WindowBuff = windowBufferPool{}
var connCopyPool, _ = ants.NewPoolWithFunc(200000, copyConnGroup, ants.WithNonblocking(false))
var CopyConnsPool, _ = ants.NewPoolWithFunc(100000, copyConns, ants.WithNonblocking(false))

func newPool() {
	BuffPool.New()
	CopyBuff.New()
	MuxPack.New()
	WindowBuff.New()
}

func init() {
	once.Do(newPool)
}
