package mux

import (
	"errors"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cnlh/nps/lib/common"
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
	remainingWait uint64 // 64bit alignment
	off           uint32
	maxSize       uint32
	closeOp       bool
	closeOpCh     chan struct{}
	mux           *Mux
}

func (Self *window) unpack(ptrs uint64) (remaining, wait uint32) {
	const mask = 1<<dequeueBits - 1
	remaining = uint32((ptrs >> dequeueBits) & mask)
	wait = uint32(ptrs & mask)
	return
}

func (Self *window) pack(remaining, wait uint32) uint64 {
	const mask = 1<<dequeueBits - 1
	return (uint64(remaining) << dequeueBits) |
		uint64(wait&mask)
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
	bufQueue ReceiveWindowQueue
	element  *common.ListElement
	count    int8
	once     sync.Once
}

func (Self *ReceiveWindow) New(mux *Mux) {
	// initial a window for receive
	Self.bufQueue.New()
	Self.element = common.ListElementPool.Get()
	Self.maxSize = 4096
	Self.mux = mux
	Self.window.New()
}

func (Self *ReceiveWindow) remainingSize(delta uint16) (n uint32) {
	// receive window remaining
	l := int64(atomic.LoadUint32(&Self.maxSize)) - int64(Self.bufQueue.Len())
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
		n := uint32(2 * Self.mux.latency * Self.mux.bw.Get() * 1.5 / float64(Self.mux.connMap.Size()))
		if n < 8192 {
			n = 8192
		}
		bufLen := Self.bufQueue.Len()
		if n < bufLen {
			n = bufLen
		}
		// set the minimal size
		if n > 2*Self.maxSize {
			n = 2 * Self.maxSize
		}
		if n > common.MAXIMUM_WINDOW_SIZE {
			n = common.MAXIMUM_WINDOW_SIZE
		}
		// set the maximum size
		//logs.Warn("n", n)
		atomic.StoreUint32(&Self.maxSize, n)
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
	//logs.Warn("push the buf", len(buf), l, (&element).l)
	if err != nil {
		return
	}
	Self.calcSize() // calculate the max window size
	var wait uint32
start:
	ptrs := atomic.LoadUint64(&Self.remainingWait)
	_, wait = Self.unpack(ptrs)
	newRemaining := Self.remainingSize(l)
	// calculate the remaining window size now, plus the element we will push
	if newRemaining == 0 {
		//logs.Warn("window full true", remaining)
		wait = 1
	}
	if !atomic.CompareAndSwapUint64(&Self.remainingWait, ptrs, Self.pack(0, wait)) {
		goto start
		// another goroutine change the status, make sure shall we need wait
	}
	Self.bufQueue.Push(element)
	// status check finish, now we can push the element into the queue
	if wait == 0 {
		Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, Self.maxSize, newRemaining)
		// send the remaining window size, not including zero size
	}
	return nil
}

func (Self *ReceiveWindow) Read(p []byte, id int32) (n int, err error) {
	if Self.closeOp {
		return 0, io.EOF // receive close signal, returns eof
	}
	pOff := 0
	l := 0
	//logs.Warn("receive window read off, element.l", Self.off, Self.element.l)
copyData:
	if Self.off == uint32(Self.element.L) {
		// on the first Read method invoked, Self.off and Self.element.l
		// both zero value
		common.ListElementPool.Put(Self.element)
		Self.element, err = Self.bufQueue.Pop()
		// if the queue is empty, Pop method will wait until one element push
		// into the queue successful, or timeout.
		// timer start on timeout parameter is set up ,
		// reset to 60s if timeout and data still available
		Self.off = 0
		if err != nil {
			return // queue receive stop or time out, break the loop and return
		}
		//logs.Warn("pop element", Self.element.l, Self.element.part)
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
	var remaining, wait uint32
	for {
		ptrs := atomic.LoadUint64(&Self.remainingWait)
		remaining, wait = Self.unpack(ptrs)
		remaining += uint32(l)
		if atomic.CompareAndSwapUint64(&Self.remainingWait, ptrs, Self.pack(remaining, 0)) {
			break
		}
		runtime.Gosched()
		// another goroutine change remaining or wait status, make sure
		// we need acknowledge other side
	}
	// now we get the current window status success
	if wait == 1 {
		//logs.Warn("send the wait status", remaining)
		Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, atomic.LoadUint32(&Self.maxSize), remaining)
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
}

type SendWindow struct {
	window
	buf       []byte
	setSizeCh chan struct{}
	timeout   time.Time
}

func (Self *SendWindow) New(mux *Mux) {
	Self.setSizeCh = make(chan struct{})
	Self.maxSize = 4096
	atomic.AddUint64(&Self.remainingWait, uint64(4096)<<dequeueBits)
	Self.mux = mux
	Self.window.New()
}

func (Self *SendWindow) SetSendBuf(buf []byte) {
	// send window buff from conn write method, set it to send window
	Self.buf = buf
	Self.off = 0
}

func (Self *SendWindow) SetSize(windowSize, newRemaining uint32) (closed bool) {
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
	var remaining, wait, newWait uint32
	for {
		ptrs := atomic.LoadUint64(&Self.remainingWait)
		remaining, wait = Self.unpack(ptrs)
		if remaining == newRemaining {
			//logs.Warn("waiting for another window size")
			return false // waiting for receive another usable window size
		}
		if newRemaining == 0 && wait == 1 {
			newWait = 1 // keep the wait status,
			// also if newRemaining is not zero, change wait to 0
		}
		if atomic.CompareAndSwapUint64(&Self.remainingWait, ptrs, Self.pack(newRemaining, newWait)) {
			break
		}
		// anther goroutine change wait status or window size
	}
	if wait == 1 {
		// send window into the wait status, need notice the channel
		//logs.Warn("send window remaining size is 0")
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
	atomic.AddUint64(&Self.remainingWait, ^(uint64(sentSize)<<dequeueBits - 1))
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
	var remaining uint32
start:
	ptrs := atomic.LoadUint64(&Self.remainingWait)
	remaining, _ = Self.unpack(ptrs)
	if remaining == 0 {
		if !atomic.CompareAndSwapUint64(&Self.remainingWait, ptrs, Self.pack(0, 1)) {
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
	//logs.Warn("rem", remaining)
	if len(Self.buf[Self.off:]) > common.MAXIMUM_SEGMENT_SIZE {
		sendSize = common.MAXIMUM_SEGMENT_SIZE
		//logs.Warn("cut buf by mss")
	} else {
		sendSize = uint32(len(Self.buf[Self.off:]))
	}
	if remaining < sendSize {
		// usable window size is small than
		// window MAXIMUM_SEGMENT_SIZE or send buf left
		sendSize = remaining
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
	if t < 0 {
		t = time.Minute
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

//type bandwidth struct {
//	readStart     time.Time
//	lastReadStart time.Time
//	readEnd       time.Time
//	lastReadEnd time.Time
//	bufLength     int
//	lastBufLength int
//	count         int8
//	readBW        float64
//	writeBW       float64
//	readBandwidth float64
//}
//
//func (Self *bandwidth) StartRead() {
//	Self.lastReadStart, Self.readStart = Self.readStart, time.Now()
//	if !Self.lastReadStart.IsZero() {
//		if Self.count == -5 {
//			Self.calcBandWidth()
//		}
//	}
//}
//
//func (Self *bandwidth) EndRead() {
//	Self.lastReadEnd, Self.readEnd = Self.readEnd, time.Now()
//	if Self.count == -5 {
//		Self.calcWriteBandwidth()
//	}
//	if Self.count == 0 {
//		Self.calcReadBandwidth()
//		Self.count = -6
//	}
//	Self.count += 1
//}
//
//func (Self *bandwidth) SetCopySize(n int) {
//	// must be invoke between StartRead and EndRead
//	Self.lastBufLength, Self.bufLength = Self.bufLength, n
//}
//// calculating
//// start end start end
////     read     read
////        write
//
//func (Self *bandwidth) calcBandWidth()  {
//	t := Self.readStart.Sub(Self.lastReadStart)
//	if Self.lastBufLength >= 32768 {
//		Self.readBandwidth = float64(Self.lastBufLength) / t.Seconds()
//	}
//}
//
//func (Self *bandwidth) calcReadBandwidth() {
//	// Bandwidth between nps and npc
//	readTime := Self.readEnd.Sub(Self.readStart)
//	Self.readBW = float64(Self.bufLength) / readTime.Seconds()
//	//logs.Warn("calc read bw", Self.readBW, Self.bufLength, readTime.Seconds())
//}
//
//func (Self *bandwidth) calcWriteBandwidth() {
//	// Bandwidth between nps and user, npc and application
//	writeTime := Self.readStart.Sub(Self.lastReadEnd)
//	Self.writeBW = float64(Self.lastBufLength) / writeTime.Seconds()
//	//logs.Warn("calc write bw", Self.writeBW, Self.bufLength, writeTime.Seconds())
//}
//
//func (Self *bandwidth) Get() (bw float64) {
//	// The zero value, 0 for numeric types
//	if Self.writeBW == 0 && Self.readBW == 0 {
//		//logs.Warn("bw both 0")
//		return 100
//	}
//	if Self.writeBW == 0 && Self.readBW != 0 {
//		return Self.readBW
//	}
//	if Self.readBW == 0 && Self.writeBW != 0 {
//		return Self.writeBW
//	}
//	return Self.readBandwidth
//}
