package nps_mux

import (
	"errors"
	"io"
	"log"
	"math"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type conn struct {
	net.Conn
	connStatusOkCh   chan struct{}
	connStatusFailCh chan struct{}
	closedCh         chan struct{} // closed by Close, wakes a NewConn still waiting for its OK
	connId           int32
	isClose          bool
	closingFlag      bool // closing conn flag
	receiveWindow    *receiveWindow
	sendWindow       *sendWindow
	once             sync.Once
}

func NewConn(connId int32, mux *Mux) *conn {
	c := &conn{
		// Buffered, and sent to without blocking: the reply to a NewConn that
		// has already given up has nobody to receive it, and the read loop
		// that delivers it must not wait for one.
		connStatusOkCh:   make(chan struct{}, 1),
		connStatusFailCh: make(chan struct{}, 1),
		closedCh:         make(chan struct{}),
		connId:           connId,
		receiveWindow:    new(receiveWindow),
		sendWindow:       new(sendWindow),
		once:             sync.Once{},
	}
	c.receiveWindow.New(mux)
	c.sendWindow.New(mux)
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
	n, err = s.receiveWindow.Read(buf, s.connId)
	return
}

func (s *conn) Write(buf []byte) (n int, err error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}
	if s.closingFlag {
		return 0, errors.New("io: write on closed conn")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	n, err = s.sendWindow.WriteFull(buf, s.connId)
	return
}

func (s *conn) Close() (err error) {
	s.once.Do(s.closeProcess)
	return
}

func (s *conn) closeProcess() {
	s.isClose = true
	close(s.closedCh)
	s.receiveWindow.mux.connMap.Delete(s.connId)
	if !s.receiveWindow.mux.IsClose {
		// if server or user close the conn while reading, will Get a io.EOF
		// and this Close method will be invoke, send this signal to close other side
		s.receiveWindow.mux.sendInfo(muxConnClose, s.connId, nil)
	}
	s.sendWindow.CloseWindow()
	s.receiveWindow.CloseWindow()
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
	if ((ptrs >> waitBits) & mask1) == 1 {
		wait = true
		return
	}
	return
}

func (Self *window) pack(maxSize, done uint32, wait bool) uint64 {
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

type receiveWindow struct {
	window
	bufQueue *receiveWindowQueue
	element  *listElement
	count    int8
	bw       *writeBandwidth
	once     sync.Once
	// receive window send the current max size and read size to send window
	// means done size actually store the size receive window has read
}

func (Self *receiveWindow) New(mux *Mux) {
	// initial a window for receive
	Self.bufQueue = newReceiveWindowQueue()
	Self.element = listEle.Get()
	Self.maxSizeDone = Self.pack(maximumSegmentSize*30, 0, false)
	Self.mux = mux
	Self.window.New()
	Self.bw = newWriteBandwidth()
}

func (Self *receiveWindow) remainingSize(maxSize uint32, delta uint16) (n uint32) {
	// receive window remaining
	l := int64(maxSize) - int64(Self.bufQueue.Len())
	l -= int64(delta)
	if l > 0 {
		n = uint32(l)
	}
	return
}

func (Self *receiveWindow) calcSize() {
	// calculating maximum receive window size
	if Self.count == 0 {
		muxBw := Self.mux.bw.Get()
		connBw := Self.bw.Get()
		latency := math.Float64frombits(atomic.LoadUint64(&Self.mux.latency))
		var n uint32
		if connBw > 0 && muxBw > 0 {
			if connBw > muxBw {
				connBw = muxBw
				Self.bw.GrowRatio()
			}
			n = uint32(latency * (muxBw + connBw))
		}
		if n < maximumSegmentSize*30 {
			n = maximumSegmentSize * 30
		}
		if n < uint32(float64(maximumSegmentSize*3000)*latency) {
			// latency gain
			// if there are some latency more than 10ms will trigger this gain
			// network pipeline need fill more data that we can measure the max bandwidth
			n = uint32(float64(maximumSegmentSize*3000) * latency)
		}
		for {
			ptrs := atomic.LoadUint64(&Self.maxSizeDone)
			size, read, wait := Self.unpack(ptrs)
			rem := Self.remainingSize(size, 0)
			ra := float64(rem) / float64(size)
			if ra > 0.8 {
				// low fill window gain
				// if receive window keep low fill, maybe pipeline fill the data, we need a gain
				// less than 20% fill, gain will trigger
				n = uint32(float64(n) * 1.5625 * ra * ra)
			}
			if n < size/2 {
				n = size / 2
				// half reduce
			}
			// set the minimal size
			if n > 2*size {
				if size == maximumSegmentSize*30 {
					// we give more ratio when the initial window size, to reduce the time window grow up
					if n > size*6 {
						n = size * 6
					}
				} else {
					n = 2 * size
					// twice grow
				}
			}
			if connBw > 0 && muxBw > 0 {
				limit := uint32(maximumWindowSize * (connBw / (muxBw + connBw)))
				if n > limit {
					log.Println("window too large, calculated:", n, "limit:", limit, connBw, muxBw)
					n = limit
				}
			}
			// set the maximum size
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

func (Self *receiveWindow) Write(buf []byte, l uint16, part bool, id int32) (err error) {
	if Self.closeOp {
		return errors.New("conn.receiveWindow: write on closed window")
	}
	element, err := newListElement(buf, l, part)
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
		wait = true
		if !atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSize, read, wait)) {
			// only change the wait status, not send the read size
			goto start
			// another goroutine change the status, make sure shall we need wait
		}
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
		Self.mux.sendInfo(muxMsgSendOk, id, Self.pack(maxSize, read, false))
		// send the current status to send window
	}
	return nil
}

func (Self *receiveWindow) Read(p []byte, id int32) (n int, err error) {
	if Self.closeOp {
		return 0, io.EOF // receive close signal, returns eof
	}
	Self.bw.StartRead()
	n, err = Self.readFromQueue(p, id)
	Self.bw.SetCopySize(uint16(n))
	return
}

func (Self *receiveWindow) readFromQueue(p []byte, id int32) (n int, err error) {
	pOff := 0
	l := 0
copyData:
	if Self.off == uint32(Self.element.L) {
		// on the first Read method invoked, Self.off and Self.element.l
		// both zero value
		listEle.Put(Self.element)
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
	}
	l = copy(p[pOff:], Self.element.Buf[Self.off:Self.element.L])
	pOff += l
	Self.off += uint32(l)
	n += l
	l = 0
	if Self.off == uint32(Self.element.L) {
		windowBuff.Put(Self.element.Buf)
		Self.sendStatus(id, Self.element.L)
		// check the window full status
	}
	if pOff < len(p) && Self.element.Part {
		// element is a part of the segments, trying to fill up buf p
		goto copyData
	}
	return // buf p is full or all of segments in buf, return
}

func (Self *receiveWindow) sendStatus(id int32, l uint16) {
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
					Self.mux.sendInfo(muxMsgSendOk, id, Self.pack(maxSize, read, false))
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
				Self.mux.sendInfo(muxMsgSendOk, id, Self.pack(maxSize, read, false))
				break
			}
		}
		runtime.Gosched()
		// another goroutine change remaining or wait status, make sure
	}
	return
}

func (Self *receiveWindow) SetTimeOut(t time.Time) {
	// waiting for FIFO queue Pop method
	Self.bufQueue.SetTimeOut(t)
}

func (Self *receiveWindow) Stop() {
	// queue has no more data to push, so unblock pop method
	Self.once.Do(Self.bufQueue.Stop)
}

func (Self *receiveWindow) CloseWindow() {
	Self.window.CloseWindow()
	Self.Stop()
	Self.release()
}

func (Self *receiveWindow) release() {
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
			windowBuff.Put(ele.Buf)
		}
		listEle.Put(ele)
	} // release resource
}

type sendWindow struct {
	window
	buf       []byte
	setSizeCh chan struct{}
	timeout   time.Time
	// send window receive the receive window max size and read size
	// done size store the size send window has send, send and read will be totally equal
	// so send minus read, send window can get the current window size remaining
}

func (Self *sendWindow) New(mux *Mux) {
	Self.setSizeCh = make(chan struct{})
	Self.maxSizeDone = Self.pack(maximumSegmentSize*30, 0, false)
	Self.mux = mux
	Self.window.New()
}

func (Self *sendWindow) SetSendBuf(buf []byte) {
	// send window buff from conn write method, set it to send window
	Self.buf = buf
	Self.off = 0
}

func (Self *sendWindow) remainingSize(maxSize, send uint32) uint32 {
	l := int64(maxSize&mask31) - int64(send&mask31)
	if l > 0 {
		return uint32(l)
	}
	return 0
}

func (Self *sendWindow) SetSize(currentMaxSizeDone uint64) (closed bool) {
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
	var maxsize, send uint32
	var wait, newWait bool
	currentMaxSize, read, _ := Self.unpack(currentMaxSizeDone)
	for {
		ptrs := atomic.LoadUint64(&Self.maxSizeDone)
		maxsize, send, wait = Self.unpack(ptrs)
		if read > send {
			log.Println("window read > send: max size:", currentMaxSize, "read:", read, "send", send)
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
			//log.Printf("currentMaxSize:%d read:%d send:%d", currentMaxSize, read, send)
			break
		}
		// anther goroutine change wait status or window size
	}
	if wait && !newWait {
		// send window into the wait status, need notice the channel
		Self.allow()
	}
	// send window not into the wait status, so just do slide
	return false
}

func (Self *sendWindow) allow() {
	select {
	case Self.setSizeCh <- struct{}{}:
		return
	case <-Self.closeOpCh:
		close(Self.setSizeCh)
		return
	}
}

func (Self *sendWindow) sent(sentSize uint32) {
	var maxSie, send uint32
	var wait bool
	for {
		ptrs := atomic.LoadUint64(&Self.maxSizeDone)
		maxSie, send, wait = Self.unpack(ptrs)
		if (send+sentSize)&mask31 < send {
			// Overflow: wait for the receive window to acknowledge and bring
			// send back down. A peer that never acknowledges leaves us here
			// indefinitely, so sleep instead of Gosched -- the latter keeps
			// the goroutine runnable and pins a CPU core at 100%.
			time.Sleep(time.Millisecond)
			continue
		}
		if atomic.CompareAndSwapUint64(&Self.maxSizeDone, ptrs, Self.pack(maxSie, send+sentSize, wait)) {
			// set the send size
			break
		}
	}
}

func (Self *sendWindow) WriteTo() (p []byte, sendSize uint32, part bool, err error) {
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
		err = Self.waitReceiveWindow()
		if err != nil {
			return nil, 0, false, err
		}
		goto start
	}
	// there are still remaining window
	if len(Self.buf[Self.off:]) > maximumSegmentSize {
		sendSize = maximumSegmentSize
	} else {
		sendSize = uint32(len(Self.buf[Self.off:]))
	}
	if remain < sendSize {
		// usable window size is small than
		// window MAXIMUM_SEGMENT_SIZE or send buf left
		sendSize = remain
	}
	if sendSize < uint32(len(Self.buf[Self.off:])) {
		part = true
	}
	p = Self.buf[Self.off : sendSize+Self.off]
	Self.off += sendSize
	Self.sent(sendSize)
	return
}

func (Self *sendWindow) waitReceiveWindow() (err error) {
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

func (Self *sendWindow) WriteFull(buf []byte, id int32) (n int, err error) {
	Self.SetSendBuf(buf) // set the buf to send window
	var bufSeg []byte
	var part bool
	var l uint32
	for {
		bufSeg, l, part, err = Self.WriteTo()
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
		var drained <-chan struct{}
		if part {
			drained = Self.mux.sendInfo(muxNewMsgPart, id, bufSeg)
		} else {
			drained = Self.mux.sendInfo(muxNewMsg, id, bufSeg)
		}
		// send to other side, not send nil data to other side
		if drained != nil {
			if err = Self.waitQueue(drained); err != nil {
				break
			}
		}
	}
	return
}

// waitQueue blocks while this stream holds more than its share of the mux's
// write queue, see sendQueue.Push.
func (Self *sendWindow) waitQueue(drained <-chan struct{}) (err error) {
	var timeout <-chan time.Time
	if !Self.timeout.IsZero() {
		timer := time.NewTimer(time.Until(Self.timeout))
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case <-drained:
		return nil
	case <-timeout:
		return errors.New("conn.writeWindow: write to time out")
	case <-Self.closeOpCh:
		return errors.New("conn.writeWindow: window closed")
	}
}

func (Self *sendWindow) SetTimeOut(t time.Time) {
	// waiting for receive a receive window size
	Self.timeout = t
}

type writeBandwidth struct {
	writeBW   uint64 // store in bits, but it's float64
	readEnd   time.Time
	duration  float64
	bufLength uint32
	ratio     uint32
}

const writeCalcThreshold uint32 = 5 * 1024 * 1024

func newWriteBandwidth() *writeBandwidth {
	return &writeBandwidth{ratio: 1}
}

func (Self *writeBandwidth) StartRead() {
	if Self.readEnd.IsZero() {
		Self.readEnd = time.Now()
	}
	Self.duration += time.Now().Sub(Self.readEnd).Seconds()
	if Self.bufLength >= writeCalcThreshold*atomic.LoadUint32(&Self.ratio) {
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

func (Self *writeBandwidth) GrowRatio() {
	atomic.AddUint32(&Self.ratio, 1)
}
