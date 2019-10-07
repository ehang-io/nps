package mux

import (
	"errors"
	"github.com/astaxie/beego/logs"
	"io"
	"math"
	"net"
	"sync"
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
}

func NewConn(connId int32, mux *Mux) *conn {
	c := &conn{
		getStatusCh:      make(chan struct{}),
		connStatusOkCh:   make(chan struct{}),
		connStatusFailCh: make(chan struct{}),
		connId:           connId,
		receiveWindow:    new(ReceiveWindow),
		sendWindow:       new(SendWindow),
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
	if s.closeFlag {
		//s.Close()
		return 0, errors.New("io: write on closed conn")
	}
	if len(buf) == 0 {
		return 0, nil
	}
	//logs.Warn("write buf", len(buf))
	n, err = s.sendWindow.WriteFull(buf, s.connId)
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
	off       uint32
	maxSize   uint32
	closeOp   bool
	closeOpCh chan struct{}
	mux       *Mux
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
	bufQueue   FIFOQueue
	element    *ListElement
	readLength uint32
	readOp     chan struct{}
	readWait   bool
	windowFull bool
	count      int8
	bw         *bandwidth
	once       sync.Once
	window
}

func (Self *ReceiveWindow) New(mux *Mux) {
	// initial a window for receive
	Self.readOp = make(chan struct{})
	Self.bufQueue.New()
	Self.bw = new(bandwidth)
	Self.element = new(ListElement)
	Self.maxSize = 8192
	Self.mux = mux
	Self.window.New()
}

func (Self *ReceiveWindow) RemainingSize() (n uint32) {
	// receive window remaining
	if Self.maxSize >= Self.bufQueue.Len() {
		n = Self.maxSize - Self.bufQueue.Len()
	}
	// if maxSize is small than bufQueue length, return 0
	return
}

func (Self *ReceiveWindow) ReadSize() (n uint32) {
	// acknowledge the size already read
	Self.bufQueue.mutex.Lock()
	n = Self.readLength
	Self.readLength = 0
	Self.bufQueue.mutex.Unlock()
	Self.count += 1
	return
}

func (Self *ReceiveWindow) CalcSize() {
	// calculating maximum receive window size
	if Self.count == 0 {
		logs.Warn("ping, bw", Self.mux.latency, Self.bw.Get())
		n := uint32(2 * Self.mux.latency * Self.bw.Get())
		if n < 8192 {
			n = 8192
		}
		if n < Self.bufQueue.Len() {
			n = Self.bufQueue.Len()
		}
		// set the minimal size
		logs.Warn("n", n)
		Self.maxSize = n
		Self.count = -5
	}
}

func (Self *ReceiveWindow) Write(buf []byte, l uint16, part bool, id int32) (err error) {
	if Self.closeOp {
		return errors.New("conn.receiveWindow: write on closed window")
	}
	element := ListElement{}
	err = element.New(buf, l, part)
	//logs.Warn("push the buf", len(buf), l, (&element).l)
	if err != nil {
		return
	}
	Self.bufQueue.Push(&element) // must push data before allow read
	//logs.Warn("read session calc size ", Self.maxSize)
	// calculating the receive window size
	Self.CalcSize()
	logs.Warn("read session calc size finish", Self.maxSize)
	if Self.RemainingSize() == 0 {
		Self.windowFull = true
		//logs.Warn("window full true", Self.windowFull)
	}
	Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, Self.maxSize, Self.ReadSize())
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
	Self.bw.StartRead()
	if Self.off == uint32(Self.element.l) {
		// on the first Read method invoked, Self.off and Self.element.l
		// both zero value
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
	l = copy(p[pOff:], Self.element.buf[Self.off:])
	Self.bw.SetCopySize(l)
	pOff += l
	Self.off += uint32(l)
	Self.bufQueue.mutex.Lock()
	Self.readLength += uint32(l)
	//logs.Warn("window read length buf len", Self.readLength, Self.bufQueue.Len())
	Self.bufQueue.mutex.Unlock()
	n += l
	l = 0
	Self.bw.EndRead()
	Self.sendStatus(id)
	if pOff < len(p) && Self.element.part {
		// element is a part of the segments, trying to fill up buf p
		goto copyData
	}
	return // buf p is full or all of segments in buf, return
}

func (Self *ReceiveWindow) sendStatus(id int32) {
	if Self.windowFull || Self.bufQueue.Len() == 0 {
		// window is full before read or empty now
		Self.windowFull = false
		Self.mux.sendInfo(common.MUX_MSG_SEND_OK, id, Self.maxSize, Self.ReadSize())
		// acknowledge other side, have empty some receive window space
		//}
	}
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
	buf         []byte
	sentLength  uint32
	setSizeCh   chan struct{}
	setSizeWait bool
	unSlide     uint32
	timeout     time.Time
	window
	mutex sync.Mutex
}

func (Self *SendWindow) New(mux *Mux) {
	Self.setSizeCh = make(chan struct{})
	Self.maxSize = 4096
	Self.mux = mux
	Self.window.New()
}

func (Self *SendWindow) SetSendBuf(buf []byte) {
	// send window buff from conn write method, set it to send window
	Self.mutex.Lock()
	Self.buf = buf
	Self.off = 0
	Self.mutex.Unlock()
}

func (Self *SendWindow) RemainingSize() (n uint32) {
	if Self.maxSize >= Self.sentLength {
		n = Self.maxSize - Self.sentLength
	}
	return
}

func (Self *SendWindow) SetSize(windowSize, readLength uint32) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()
	if Self.closeOp {
		close(Self.setSizeCh)
		return true
	}
	if readLength == 0 && Self.maxSize == windowSize {
		logs.Warn("waiting for another window size")
		return false // waiting for receive another usable window size
	}
	logs.Warn("set send window size to ", windowSize, readLength)
	Self.mutex.Lock()
	Self.slide(windowSize, readLength)
	if Self.setSizeWait {
		// send window into the wait status, need notice the channel
		//logs.Warn("send window remaining size is 0 , wait")
		if Self.RemainingSize() == 0 {
			//logs.Warn("waiting for another window size after slide")
			// keep the wait status
			Self.mutex.Unlock()
			return false
		}
		Self.setSizeWait = false
		Self.mutex.Unlock()
		//logs.Warn("send window remaining size is 0 starting wait")
		select {
		case Self.setSizeCh <- struct{}{}:
			//logs.Warn("send window remaining size is 0 finish")
			return false
		case <-Self.closeOpCh:
			close(Self.setSizeCh)
			return true
		}
	}
	// send window not into the wait status, so just do slide
	Self.mutex.Unlock()
	return false
}

func (Self *SendWindow) slide(windowSize, readLength uint32) {
	Self.sentLength -= readLength
	Self.maxSize = windowSize
}

func (Self *SendWindow) WriteTo() (p []byte, part bool, err error) {
	// returns buf segments, return only one segments, need a loop outside
	// until err = io.EOF
	if Self.closeOp {
		return nil, false, errors.New("conn.writeWindow: window closed")
	}
	if Self.off == uint32(len(Self.buf)) {
		return nil, false, io.EOF
		// send window buff is drain, return eof and get another one
	}
	Self.mutex.Lock()
	if Self.RemainingSize() == 0 {
		Self.setSizeWait = true
		Self.mutex.Unlock()
		// into the wait status
		err = Self.waitReceiveWindow()
		if err != nil {
			return nil, false, err
		}
	} else {
		Self.mutex.Unlock()
	}
	Self.mutex.Lock()
	var sendSize uint32
	if len(Self.buf[Self.off:]) > common.MAXIMUM_SEGMENT_SIZE {
		sendSize = common.MAXIMUM_SEGMENT_SIZE
		part = true
	} else {
		sendSize = uint32(len(Self.buf[Self.off:]))
		part = false
	}
	if Self.RemainingSize() < sendSize {
		// usable window size is small than
		// window MAXIMUM_SEGMENT_SIZE or send buf left
		sendSize = Self.RemainingSize()
		part = true
	}
	//logs.Warn("send size", sendSize)
	p = Self.buf[Self.off : sendSize+Self.off]
	Self.off += sendSize
	Self.sentLength += sendSize
	Self.mutex.Unlock()
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
	var bufSeg []byte
	var part bool
	for {
		bufSeg, part, err = Self.WriteTo()
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
		n += len(bufSeg)
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

type bandwidth struct {
	lastReadStart time.Time
	readStart     time.Time
	readEnd       time.Time
	bufLength     int
	lastBufLength int
	count         int8
	readBW        float64
	writeBW       float64
}

func (Self *bandwidth) StartRead() {
	Self.lastReadStart, Self.readStart = Self.readStart, time.Now()
}

func (Self *bandwidth) EndRead() {
	if !Self.lastReadStart.IsZero() {
		if Self.count == 0 {
			Self.calcWriteBandwidth()
		}
	}
	Self.readEnd = time.Now()
	if Self.count == 0 {
		Self.calcReadBandwidth()
		Self.count = -3
	}
	Self.count += 1
}

func (Self *bandwidth) SetCopySize(n int) {
	// must be invoke between StartRead and EndRead
	Self.lastBufLength, Self.bufLength = Self.bufLength, n
}

func (Self *bandwidth) calcReadBandwidth() {
	// Bandwidth between nps and npc
	readTime := Self.readEnd.Sub(Self.readStart)
	Self.readBW = float64(Self.bufLength) / readTime.Seconds()
	//logs.Warn("calc read bw", Self.bufLength, readTime.Seconds())
}

func (Self *bandwidth) calcWriteBandwidth() {
	// Bandwidth between nps and user, npc and application
	//logs.Warn("calc write bw")
	writeTime := Self.readEnd.Sub(Self.lastReadStart)
	Self.writeBW = float64(Self.lastBufLength) / writeTime.Seconds()
}

func (Self *bandwidth) Get() (bw float64) {
	// The zero value, 0 for numeric types
	if Self.writeBW == 0 && Self.readBW == 0 {
		logs.Warn("bw both 0")
		return 100
	}
	if Self.writeBW == 0 && Self.readBW != 0 {
		return Self.readBW
	}
	if Self.readBW == 0 && Self.writeBW != 0 {
		return Self.writeBW
	}
	return math.Min(Self.readBW, Self.writeBW)
}
