package mux

import (
	"ehang.io/nps/lib/common"
	"errors"
	"github.com/astaxie/beego/logs"
	"io"
	"math"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type conn struct {
	net.Conn
	getStatusCh      chan struct{}
	connStatusOkCh   chan struct{}
	connStatusFailCh chan struct{}
	connId           int32
	isClose          bool
	closeFlag        bool // close conn flag
	receiveWindow    *ReceiveWindow
	sendWindow       *SendWindow
	once             sync.Once
	//label            string
}

func NewConn(connId int32, mux *Mux, label ...string) *conn {
	c := &conn{
		getStatusCh:      make(chan struct{}),
		connStatusOkCh:   make(chan struct{}),
		connStatusFailCh: make(chan struct{}),
		connId:           connId,
		receiveWindow:    new(ReceiveWindow),
		sendWindow:       new(SendWindow),
		once:             sync.Once{},
	}
	//if len(label) > 0 {
	//	c.label = label[0]
	//}
	c.receiveWindow.New(mux)
	c.sendWindow.New(mux)
	//logm := &connLog{
	//	startTime: time.Now(),
	//	isClose:   false,
	//	logs:      []string{c.label + "new conn success"},
	//}
	//setM(label[0], int(connId), logm)
	return c
}

func (s *conn) Read(buf []byte) (n int, err error) {
	if s.isClose || buf == nil {
		return 0, errors.New("the conn has closed")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	// waiting for takeout from receive window finish or timeout
	//now := time.Now()
	n, err = s.receiveWindow.Read(buf, s.connId)
	//t := time.Now().Sub(now)
	//if t.Seconds() > 0.5 {
	//logs.Warn("conn read long", n, t.Seconds())
	//}
	//var errstr string
	//if err == nil {
	//	errstr = "err:nil"
	//} else {
	//	errstr = err.Error()
	//}
	//d := getM(s.label, int(s.connId))
	//d.logs = append(d.logs, s.label+"read "+strconv.Itoa(n)+" "+errstr+" "+string(buf[:100]))
	//setM(s.label, int(s.connId), d)
	return
}

func (s *conn) Write(buf []byte) (n int, err error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}
	if s.closeFlag {
		//s.Close()
		return 0, errors.New("io: write on closed conn")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	//logs.Warn("write buf", len(buf))
	//now := time.Now()
	n, err = s.sendWindow.WriteFull(buf, s.connId)
	//t := time.Now().Sub(now)
	//if t.Seconds() > 0.5 {
	//	logs.Warn("conn write long", n, t.Seconds())
	//}
	return
}

func (s *conn) Close() (err error) {
	s.once.Do(s.closeProcess)
	return
}

func (s *conn) closeProcess() {
	s.isClose = true
	s.receiveWindow.mux.connMap.Delete(s.connId)
	if !s.receiveWindow.mux.IsClose {
		// if server or user close the conn while reading, will get a io.EOF
		// and this Close method will be invoke, send this signal to close other side
		s.receiveWindow.mux.sendInfo(common.MUX_CONN_CLOSE, s.connId, nil)
	}
	s.sendWindow.CloseWindow()
	s.receiveWindow.CloseWindow()
	//d := getM(s.label, int(s.connId))
	//d.isClose = true
	//d.logs = append(d.logs, s.label+"close "+time.Now().String())
	//setM(s.label, int(s.connId), d)
	return
}

func (s *conn) LocalAddr() net.Addr {
	return s.receiveWindow.mux.conn.LocalAddr()
}

func (s *conn) RemoteAddr() net.Addr {
	return s.receiveWindow.mux.conn.RemoteAddr()
}

func (s *conn) SetDeadline(t time.Time) error {
	_ = s.SetReadDeadline(t)
	_ = s.SetWriteDeadline(t)
	return nil
}

func (s *conn) SetReadDeadline(t time.Time) error {
	s.receiveWindow.SetTimeOut(t)
	return nil
}

func (s *conn) SetWriteDeadline(t time.Time) error {
	s.sendWindow.SetTimeOut(t)
	return nil
}

type window struct {
	maxSizeDone uint64
	// 64bit alignment
	// maxSizeDone contains 4 parts
	//   1       31       1      31
	// wait   maxSize  useless  done
	// wait zero means false, one means true
	off       uint32
	closeOp   bool
	closeOpCh chan struct{}
	mux       *Mux
}

const windowBits = 31
const waitBits = dequeueBits + windowBits
const mask1 = 1
const mask31 = 1<<windowBits - 1

func (Self *window) unpack(ptrs uint64) (maxSize, done uint32, wait bool) {
	maxSize = uint32((ptrs >> dequeueBits) & mask31)
	done = uint32(ptrs & mask31)
	//logs.Warn("unpack", maxSize, done)
	if ((ptrs >> waitBits) & mask1) == 1 {
		wait = true
		return
	}
	return
}

func (Self *window) pack(maxSize, done uint32, wait bool) uint64 {
	//logs.Warn("pack", maxSize, done, wait)
	if wait {
		return (uint64(1)<<waitBits |
			uint64(maxSize&mask31)<<dequeueBits) |
			uint64(done&mask31)
	}
	return (uint64(0)<<waitBits |
		uint64(maxSize&mask31)<<dequeueBits) |
		uint64(done&mask31)
}

func (Self *window) New() {
	Self.closeOpCh = make(chan struct{}, 2)
}

func (Self *window) CloseWindow() {
	if !Self.closeOp {
		Self.closeOp = true
		Self.closeOpCh <- struct{}{}
		Self.closeOpCh <- struct{}{}
	}
}

type ReceiveWindow struct {
	window
	bufQueue *ReceiveWindowQueue
	element  *common.ListElement
	count    int8
	bw       *writeBandwidth
	once     sync.Once
	// receive window send the current max size and read size to send window
	// means done size actually store the size receive window has read
}

func (Self *ReceiveWindow) New(mux *Mux) {
	// initial a window for receive
	Self.bufQueue = NewReceiveWindowQueue()
	Self.element = common.ListElementPool.Get()
	Self.maxSizeDone = Self.pack(common.MAXIMUM_SEGMENT_SIZE*30, 0, false)
	Self.mux = mux
	Self.window.New()
	Self.bw = NewWriteBandwidth()
}

func (Self *ReceiveWindow) remainingSize(maxSize uint32, delta uint16) (n uint32) {
	// receive window remaining
	l := int64(maxSize) - int64(Self.bufQueue.Len())
	l -= int64(delta)
	if l > 0 {
		n = uint32(l)
	}
	return
}

func (Self *ReceiveWindow) calcSize() {
	// calculating maximum receive window size
	if Self.count == 0 {
		//logs.Warn("ping, bw", Self.mux.latency, Self.bw.Get())
		//conns := Self.mux.connMap.Size()
		muxBw := Self.mux.bw.Get()
		connBw := Self.bw.Get()
		//logs.Warn("muxbw connbw", muxBw, connBw)
		var n uint32
		if connBw > 0 && muxBw > 0 {
			n = uint32(math.Float64frombits(atomic.LoadUint64(&Self.mux.latency)) *
				(muxBw + connBw))
		}
		//logs.Warn(n)
		if n < common.MAXIMUM_SEGMENT_SIZE*30 {
			//logs.Warn("window small", n, Self.mux.bw.Get(), Self.bw.Get())
			n = common.MAXIMUM_SEGMENT_SIZE * 30
		}
		for {
			ptrs := atomic.LoadUint64(&Self.maxSizeDone)
			size, read, wait := Self.unpack(ptrs)
			if n < size/2 {
				n = size / 2
				// half reduce
			}
			// set the minimal size
			if n > 2*size {
				n = 2 * size
				// twice grow
			}
			if connBw > 0 && muxBw > 0 {
				limit := uint32(common.MAXIMUM_WINDOW_SIZE * (connBw / (muxBw + connBw)))
				if n > limit {
					logs.Warn("window too large, calculated:", n, "limit:", limit, connBw, muxBw)
					n = limit
				}
			}
			// set the maximum size
			//logs.Warn("n", n)
			if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(n, read, wait)) {
				// only change the maxSize
				break
			}
		}
		Self.count = -10
	}
	Self.count += 1
	return
}

func (Self *ReceiveWindow) Write(buf []byte, l uint16, part bool, id int32) (err error) {
	if Self.closeOp {
		return errors.New("conn.receiveWindow: write on closed window")
	}
	element, err := NewListElement(buf, l, part)
	//logs.Warn("push the buf", len(buf), l, element.L)
	if err != nil {
		return
	}
	Self.calcSize() // calculate the max window size
	var wait bool
	var maxSize, read uint32
start:
	ptrs := atomic.LoadUint64(&Self.maxSizeDone)
	maxSize, read, wait = Self.unpack(ptrs)
	remain := Self.remainingSize(maxSize, l)
	// calculate the remaining window size now, plus the element we will push
	if remain == 0 && !wait {
		//logs.Warn("window full true", remaining)
		wait = true
		if !atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, read, wait)) {
			// only change the wait status, not send the read size
			goto start
			// another goroutine change the status, make sure shall we need wait
		}
		//logs.Warn("receive window full")
	} else if !wait {
		if !atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, 0, wait)) {
			// reset read size here, and send the read size directly
			goto start
			// another goroutine change the status, make sure shall we need wait
		}
	} // maybe there are still some data received even if window is full, just keep the wait status
	// and push into queue. when receive window read enough, send window will be acknowledged.
	Self.bufQueue.Push(element)
	// status check finish, now we can push the element into the queue
	if !wait {
		Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, Self.pack(maxSize, read, false))
		// send the current status to send window
	}
	return nil
}

func (Self *ReceiveWindow) Read(p []byte, id int32) (n int, err error) {
	if Self.closeOp {
		return 0, io.EOF // receive close signal, returns eof
	}
	Self.bw.StartRead()
	n, err = Self.readFromQueue(p, id)
	Self.bw.SetCopySize(uint16(n))
	return
}

func (Self *ReceiveWindow) readFromQueue(p []byte, id int32) (n int, err error) {
	pOff := 0
	l := 0
	//logs.Warn("receive window read off, element.l", Self.off, Self.element.L)
copyData:
	if Self.off == uint32(Self.element.L) {
		// on the first Read method invoked, Self.off and Self.element.l
		// both zero value
		common.ListElementPool.Put(Self.element)
		if Self.closeOp {
			return 0, io.EOF
		}
		Self.element, err = Self.bufQueue.Pop()
		// if the queue is empty, Pop method will wait until one element push
		// into the queue successful, or timeout.
		// timer start on timeout parameter is set up
		Self.off = 0
		if err != nil {
			Self.CloseWindow() // also close the window, to avoid read twice
			return             // queue receive stop or time out, break the loop and return
		}
		//logs.Warn("pop element", Self.element.L, Self.element.Part)
	}
	l = copy(p[pOff:], Self.element.Buf[Self.off:Self.element.L])
	pOff += l
	Self.off += uint32(l)
	//logs.Warn("window read length buf len", Self.readLength, Self.bufQueue.Len())
	n += l
	l = 0
	if Self.off == uint32(Self.element.L) {
		//logs.Warn("put the element end ", string(Self.element.buf[:15]))
		common.WindowBuff.Put(Self.element.Buf)
		Self.sendStatus(id, Self.element.L)
		// check the window full status
	}
	if pOff < len(p) && Self.element.Part {
		// element is a part of the segments, trying to fill up buf p
		goto copyData
	}
	return // buf p is full or all of segments in buf, return
}

func (Self *ReceiveWindow) sendStatus(id int32, l uint16) {
	var maxSize, read uint32
	var wait bool
	for {
		ptrs := atomic.LoadUint64(&Self.maxSizeDone)
		maxSize, read, wait = Self.unpack(ptrs)
		if read <= (read+uint32(l))&mask31 {
			read += uint32(l)
			remain := Self.remainingSize(maxSize, 0)
			if wait && remain > 0 || read >= maxSize/2 || remain == maxSize {
				if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, 0, false)) {
					// now we get the current window status success
					// receive window free up some space we need acknowledge send window, also reset the read size
					// still having a condition that receive window is empty and not send the status to send window
					// so send the status here
					//logs.Warn("receive window free up some space", remain)
					Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, Self.pack(maxSize, read, false))
					break
				}
			} else {
				if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, read, wait)) {
					// receive window not into the wait status, or still not having any space now,
					// just change the read size
					break
				}
			}
		} else {
			//overflow
			if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, uint32(l), wait)) {
				// reset to l
				Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, Self.pack(maxSize, read, false))
				break
			}
		}
		runtime.Gosched()
		// another goroutine change remaining or wait status, make sure
	}
	return
}

func (Self *ReceiveWindow) SetTimeOut(t time.Time) {
	// waiting for FIFO queue Pop method
	Self.bufQueue.SetTimeOut(t)
}

func (Self *ReceiveWindow) Stop() {
	// queue has no more data to push, so unblock pop method
	Self.once.Do(Self.bufQueue.Stop)
}

func (Self *ReceiveWindow) CloseWindow() {
	Self.window.CloseWindow()
	Self.Stop()
	Self.release()
}

func (Self *ReceiveWindow) release() {
	//if Self.element != nil {
	//	if Self.element.Buf != nil {
	//		common.WindowBuff.Put(Self.element.Buf)
	//	}
	//	common.ListElementPool.Put(Self.element)
	//}
	for {
		ele := Self.bufQueue.TryPop()
		if ele == nil {
			return
		}
		if ele.Buf != nil {
			common.WindowBuff.Put(ele.Buf)
		}
		common.ListElementPool.Put(ele)
	} // release resource
}

type SendWindow struct {
	window
	buf       []byte
	setSizeCh chan struct{}
	timeout   time.Time
	// send window receive the receive window max size and read size
	// done size store the size send window has send, send and read will be totally equal
	// so send minus read, send window can get the current window size remaining
}

func (Self *SendWindow) New(mux *Mux) {
	Self.setSizeCh = make(chan struct{})
	Self.maxSizeDone = Self.pack(common.MAXIMUM_SEGMENT_SIZE*30, 0, false)
	Self.mux = mux
	Self.window.New()
}

func (Self *SendWindow) SetSendBuf(buf []byte) {
	// send window buff from conn write method, set it to send window
	Self.buf = buf
	Self.off = 0
}

func (Self *SendWindow) remainingSize(maxSize, send uint32) uint32 {
	l := int64(maxSize&mask31) - int64(send&mask31)
	if l > 0 {
		return uint32(l)
	}
	return 0
}

func (Self *SendWindow) SetSize(currentMaxSizeDone uint64) (closed bool) {
	// set the window size from receive window
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()
	if Self.closeOp {
		close(Self.setSizeCh)
		return true
	}
	//logs.Warn("set send window size to ", windowSize, newRemaining)
	var maxsize, send uint32
	var wait, newWait bool
	currentMaxSize, read, _ := Self.unpack(currentMaxSizeDone)
	for {
		ptrs := atomic.LoadUint64(&Self.maxSizeDone)
		maxsize, send, wait = Self.unpack(ptrs)
		if read > send {
			logs.Error("window read > send: max size:", currentMaxSize, "read:", read, "send", send)
			return
		}
		if read == 0 && currentMaxSize == maxsize {
			return
		}
		send -= read
		remain := Self.remainingSize(currentMaxSize, send)
		if remain == 0 && wait {
			// just keep the wait status
			newWait = true
		}
		// remain > 0, change wait to false. or remain == 0, wait is false, just keep it
		if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(currentMaxSize, send, newWait)) {
			break
		}
		// anther goroutine change wait status or window size
	}
	if wait && !newWait {
		// send window into the wait status, need notice the channel
		//logs.Warn("send window allow")
		Self.allow()
	}
	// send window not into the wait status, so just do slide
	return false
}

func (Self *SendWindow) allow() {
	select {
	case Self.setSizeCh <- struct{}{}:
		//logs.Warn("send window remaining size is 0 finish")
		return
	case <-Self.closeOpCh:
		close(Self.setSizeCh)
		return
	}
}

func (Self *SendWindow) sent(sentSize uint32) {
	var maxSie, send uint32
	var wait bool
	for {
		ptrs := atomic.LoadUint64(&Self.maxSizeDone)
		maxSie, send, wait = Self.unpack(ptrs)
		if (send+sentSize)&mask31 < send {
			// overflow
			runtime.Gosched()
			continue
		}
		if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSie, send+sentSize, wait)) {
			// set the send size
			//logs.Warn("sent", maxSie, send+sentSize, wait)
			break
		}
	}
}

func (Self *SendWindow) WriteTo() (p []byte, sendSize uint32, part bool, err error) {
	// returns buf segments, return only one segments, need a loop outside
	// until err = io.EOF
	if Self.closeOp {
		return nil, 0, false, errors.New("conn.writeWindow: window closed")
	}
	if Self.off == uint32(len(Self.buf)) {
		return nil, 0, false, io.EOF
		// send window buff is drain, return eof and get another one
	}
	var maxSize, send uint32
start:
	ptrs := atomic.LoadUint64(&Self.maxSizeDone)
	maxSize, send, _ = Self.unpack(ptrs)
	remain := Self.remainingSize(maxSize, send)
	if remain == 0 {
		if !atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, send, true)) {
			// just change the status wait status
			goto start // another goroutine change the window, try again
		}
		// into the wait status
		//logs.Warn("send window into wait status")
		err = Self.waitReceiveWindow()
		if err != nil {
			return nil, 0, false, err
		}
		//logs.Warn("rem into wait finish")
		goto start
	}
	// there are still remaining window
	//logs.Warn("rem", remain, maxSize, send)
	if len(Self.buf[Self.off:]) > common.MAXIMUM_SEGMENT_SIZE {
		sendSize = common.MAXIMUM_SEGMENT_SIZE
		//logs.Warn("cut buf by mss")
	} else {
		sendSize = uint32(len(Self.buf[Self.off:]))
	}
	if remain < sendSize {
		// usable window size is small than
		// window MAXIMUM_SEGMENT_SIZE or send buf left
		sendSize = remain
		//logs.Warn("cut buf by remainingsize", sendSize, len(Self.buf[Self.off:]))
	}
	//logs.Warn("send size", sendSize)
	if sendSize < uint32(len(Self.buf[Self.off:])) {
		part = true
	}
	p = Self.buf[Self.off : sendSize+Self.off]
	Self.off += sendSize
	Self.sent(sendSize)
	return
}

func (Self *SendWindow) waitReceiveWindow() (err error) {
	t := Self.timeout.Sub(time.Now())
	if t < 0 { // not set the timeout, wait for it as long as connection close
		select {
		case _, ok := <-Self.setSizeCh:
			if !ok {
				return errors.New("conn.writeWindow: window closed")
			}
			return nil
		case <-Self.closeOpCh:
			return errors.New("conn.writeWindow: window closed")
		}
	}
	timer := time.NewTimer(t)
	defer timer.Stop()
	// waiting for receive usable window size, or timeout
	select {
	case _, ok := <-Self.setSizeCh:
		if !ok {
			return errors.New("conn.writeWindow: window closed")
		}
		return nil
	case <-timer.C:
		return errors.New("conn.writeWindow: write to time out")
	case <-Self.closeOpCh:
		return errors.New("conn.writeWindow: window closed")
	}
}

func (Self *SendWindow) WriteFull(buf []byte, id int32) (n int, err error) {
	Self.SetSendBuf(buf) // set the buf to send window
	//logs.Warn("set the buf to send window")
	var bufSeg []byte
	var part bool
	var l uint32
	for {
		bufSeg, l, part, err = Self.WriteTo()
		//logs.Warn("buf seg", len(bufSeg), part, err)
		// get the buf segments from send window
		if bufSeg == nil && part == false && err == io.EOF {
			// send window is drain, break the loop
			err = nil
			break
		}
		if err != nil {
			break
		}
		n += int(l)
		l = 0
		if part {
			Self.mux.sendInfo(common.MUX_NEW_MSG_PART, id, bufSeg)
		} else {
			Self.mux.sendInfo(common.MUX_NEW_MSG, id, bufSeg)
			//logs.Warn("buf seg sent", len(bufSeg), part, err)
		}
		// send to other side, not send nil data to other side
	}
	//logs.Warn("buf seg write success")
	return
}

func (Self *SendWindow) SetTimeOut(t time.Time) {
	// waiting for receive a receive window size
	Self.timeout = t
}

type writeBandwidth struct {
	writeBW   uint64 // store in bits, but it's float64
	readEnd   time.Time
	duration  float64
	bufLength uint32
}

const writeCalcThreshold uint32 = 5 * 1024 * 1024

func NewWriteBandwidth() *writeBandwidth {
	return &writeBandwidth{}
}

func (Self *writeBandwidth) StartRead() {
	if Self.readEnd.IsZero() {
		Self.readEnd = time.Now()
	}
	Self.duration += time.Now().Sub(Self.readEnd).Seconds()
	if Self.bufLength >= writeCalcThreshold {
		Self.calcBandWidth()
	}
}

func (Self *writeBandwidth) SetCopySize(n uint16) {
	Self.bufLength += uint32(n)
	Self.endRead()
}

func (Self *writeBandwidth) endRead() {
	Self.readEnd = time.Now()
}

func (Self *writeBandwidth) calcBandWidth() {
	atomic.StoreUint64(&Self.writeBW, math.Float64bits(float64(Self.bufLength)/Self.duration))
	Self.bufLength = 0
	Self.duration = 0
}

func (Self *writeBandwidth) Get() (bw float64) {
	// The zero value, 0 for numeric types
	bw = math.Float64frombits(atomic.LoadUint64(&Self.writeBW))
	if bw <= 0 {
		bw = 0
	}
	return
}
