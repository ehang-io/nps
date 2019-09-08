package mux

import (
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"io"
	"net"
	"sync"
	"time"
)

type conn struct {
	net.Conn
	getStatusCh      chan struct{}
	connStatusOkCh   chan struct{}
	connStatusFailCh chan struct{}
	readTimeOut      time.Time
	writeTimeOut     time.Time
	//readBuffer       []byte
	//startRead        int //now read position
	//endRead          int //now end read
	//readFlag         bool
	//readCh           chan struct{}
	//readQueue        *sliceEntry
	//stopWrite        bool
	connId  int32
	isClose bool
	//readWait         bool
	closeFlag     bool // close conn flag
	hasWrite      int
	receiveWindow *window
	sendWindow    *window
	mux           *Mux
	once          sync.Once
}

func NewConn(connId int32, mux *Mux) *conn {
	c := &conn{
		getStatusCh:      make(chan struct{}),
		connStatusOkCh:   make(chan struct{}),
		connStatusFailCh: make(chan struct{}),
		connId:           connId,
		receiveWindow:    new(window),
		sendWindow:       new(window),
		mux:              mux,
		once:             sync.Once{},
	}
	c.receiveWindow.NewReceive()
	c.sendWindow.NewSend()
	return c
}

func (s *conn) Read(buf []byte) (n int, err error) {
	if s.isClose || buf == nil {
		return 0, errors.New("the conn has closed")
	}
	nCh := make(chan int)
	errCh := make(chan error)
	defer close(nCh)
	defer close(errCh)
	// waiting for takeout from receive window finish or timeout
	go s.readWindow(buf, nCh, errCh)
	if t := s.readTimeOut.Sub(time.Now()); t > 0 {
		timer := time.NewTimer(t)
		defer timer.Stop()
		select {
		case <-timer.C:
			return 0, errors.New("read timeout")
		case n = <-nCh:
			err = <-errCh
		}
	} else {
		n = <-nCh
		err = <-errCh
	}
	logs.Warn("read window finish conn read n err buf", n, err, string(buf[:15]))
	return
}

func (s *conn) readWindow(buf []byte, nCh chan int, errCh chan error) {
	n, err := s.receiveWindow.Read(buf)
	//logs.Warn("readwindow goroutine status n err buf", n, err, string(buf[:15]))
	if s.receiveWindow.WindowFull {
		if s.receiveWindow.Size() > 0 {
			// window.Read may be invoked before window.Write, and WindowFull flag change to true
			// so make sure that receiveWindow is free some space
			s.receiveWindow.WindowFull = false
			logs.Warn("defer send mux msg send ok size", s.receiveWindow.Size())
			s.mux.sendInfo(common.MUX_MSG_SEND_OK, s.connId, s.receiveWindow.Size())
			// acknowledge other side, have empty some receive window space
		}
	}
	nCh <- n
	errCh <- err
}

func (s *conn) Write(buf []byte) (n int, err error) {
	if s.isClose {
		return 0, errors.New("the conn has closed")
	}
	if s.closeFlag {
		logs.Warn("conn close by write ", s.connId)
		//s.Close()
		return 0, errors.New("io: write on closed conn")
	}

	nCh := make(chan int)
	errCh := make(chan error)
	defer close(nCh)
	defer close(errCh)
	s.sendWindow.SetSendBuf(buf) // set the buf to send window
	go s.write(nCh, errCh)
	// waiting for send to other side or timeout
	if t := s.writeTimeOut.Sub(time.Now()); t > 0 {
		timer := time.NewTimer(t)
		defer timer.Stop()
		select {
		case <-timer.C:
			return 0, errors.New("write timeout")
		case n = <-nCh:
			err = <-errCh
		}
	} else {
		n = <-nCh
		err = <-errCh
	}
	return
}
func (s *conn) write(nCh chan int, errCh chan error) {
	var n int
	var err error
	for {
		buf, err := s.sendWindow.WriteTo()
		// get the usable window size buf from send window
		if buf == nil && err == io.EOF {
			// send window is drain, break the loop
			err = nil
			break
		}
		if err != nil {
			break
		}
		n += len(buf)
		//logs.Warn("send window buf len", len(buf))
		s.mux.sendInfo(common.MUX_NEW_MSG, s.connId, buf)
		// send to other side, not send nil data to other side
	}
	nCh <- n
	errCh <- err
}

func (s *conn) Close() (err error) {
	s.once.Do(s.closeProcess)
	return
}

func (s *conn) closeProcess() {
	s.isClose = true
	s.mux.connMap.Delete(s.connId)
	if !s.mux.IsClose {
		logs.Warn("conn send close", s.connId)
		// if server or user close the conn while reading, will get a io.EOF
		// and this Close method will be invoke, send this signal to close other side
		s.mux.sendInfo(common.MUX_CONN_CLOSE, s.connId, nil)
	}
	s.sendWindow.CloseWindow()
	s.receiveWindow.CloseWindow()
	return
}

func (s *conn) LocalAddr() net.Addr {
	return s.mux.conn.LocalAddr()
}

func (s *conn) RemoteAddr() net.Addr {
	return s.mux.conn.RemoteAddr()
}

func (s *conn) SetDeadline(t time.Time) error {
	s.readTimeOut = t
	s.writeTimeOut = t
	return nil
}

func (s *conn) SetReadDeadline(t time.Time) error {
	s.readTimeOut = t
	return nil
}

func (s *conn) SetWriteDeadline(t time.Time) error {
	s.writeTimeOut = t
	return nil
}

type window struct {
	windowBuff          []byte
	off                 uint16
	readOp              chan struct{}
	readWait            bool
	WindowFull          bool
	usableReceiveWindow chan uint16
	WriteWg             sync.WaitGroup
	closeOp             bool
	closeOpCh           chan struct{}
	WriteEndOp          chan struct{}
	mutex               sync.Mutex
}

func (Self *window) NewReceive() {
	// initial a window for receive
	Self.windowBuff = common.WindowBuff.Get()
	Self.readOp = make(chan struct{})
	Self.WriteEndOp = make(chan struct{})
	Self.closeOpCh = make(chan struct{}, 2)
}

func (Self *window) NewSend() {
	// initial a window for send
	Self.usableReceiveWindow = make(chan uint16)
	Self.closeOpCh = make(chan struct{}, 2)
}

func (Self *window) SetSendBuf(buf []byte) {
	// send window buff from conn write method, set it to send window
	Self.mutex.Lock()
	Self.windowBuff = buf
	Self.off = 0
	Self.mutex.Unlock()
}

func (Self *window) fullSlide() {
	// slide by allocate
	newBuf := common.WindowBuff.Get()
	copy(newBuf[0:Self.len()], Self.windowBuff[Self.off:])
	Self.off = 0
	common.WindowBuff.Put(Self.windowBuff)
	Self.windowBuff = newBuf
	return
}

func (Self *window) liteSlide() {
	// slide by re slice
	Self.windowBuff = Self.windowBuff[Self.off:]
	Self.off = 0
	return
}

func (Self *window) Size() (n int) {
	// receive Window remaining
	n = common.PoolSizeWindow - Self.len()
	return
}

func (Self *window) len() (n int) {
	n = len(Self.windowBuff[Self.off:])
	return
}

func (Self *window) cap() (n int) {
	n = cap(Self.windowBuff[Self.off:])
	return
}

func (Self *window) grow(n int) {
	Self.windowBuff = Self.windowBuff[:Self.len()+n]
}

func (Self *window) Write(p []byte) (n int, err error) {
	if Self.closeOp {
		logs.Warn("window write closed len p", len(p))
		return 0, errors.New("conn.receiveWindow: write on closed window")
	}
	if len(p) > Self.Size() {
		return 0, errors.New("conn.receiveWindow: write too large")
	}
	if Self.readWait {
		defer Self.allowRead()
	}
	//logs.Warn("window write p string", len(p), string(p[:15]))
	Self.mutex.Lock()
	// slide the offset
	if len(p) > Self.cap()-Self.len() {
		// not enough space, need to allocate
		Self.fullSlide()
		//logs.Warn("window write full slide len cap", Self.len(), Self.cap())
	} else {
		// have enough space, re slice
		Self.liteSlide()
		//logs.Warn("window write lite slide len cap", Self.len(), Self.cap())
	}
	length := Self.len()                  // length before grow
	Self.grow(len(p))                     // grow for copy
	n = copy(Self.windowBuff[length:], p) // must copy data before allow Read
	//logs.Warn("window write copy n len cap buf", n, Self.len(), Self.cap(), string(Self.windowBuff[Self.len()-n:Self.len()+15-n]))
	Self.mutex.Unlock()
	return n, nil
}

func (Self *window) allowRead() (closed bool) {
	//logs.Warn("length 0 read op")
	Self.readWait = false
	if Self.closeOp {
		close(Self.readOp)
		return true
	}
	select {
	case <-Self.closeOpCh:
		close(Self.readOp)
		return true
	case Self.readOp <- struct{}{}:
		//logs.Warn("length 0 read op finish")
		return false
	}
}

func (Self *window) Read(p []byte) (n int, err error) {
	//logs.Warn("starting window read method len ", Self.len())
	if Self.closeOp {
		return 0, io.EOF // Write method receive close signal, returns eof
	}
	if Self.len() == 0 {
		// window is empty, waiting for Write method send a success readOp signal
		// or get timeout or close
		Self.readWait = true
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		select {
		case _, ok := <-Self.readOp:
			//logs.Warn("read window read op len cap", Self.len(), Self.cap())
			if !ok {
				return 0, errors.New("conn.receiveWindow: window closed")
			}
		case <-Self.WriteEndOp:
			return 0, io.EOF // receive eof signal, returns eof
		case <-ticker.C:
			return 0, errors.New("conn.receiveWindow: read time out")
		case <-Self.closeOpCh:
			close(Self.readOp)
			return 0, io.EOF // receive close signal, returns eof
		}
	}
	//logs.Warn("window read start len window buff", Self.len(), string(Self.windowBuff[Self.off:Self.off+15]))
	Self.mutex.Lock()
	n = copy(p, Self.windowBuff[Self.off:])
	Self.off += uint16(n)
	p = p[:n]
	//logs.Warn("window read finish n len p p", n, len(p), string(p[:15]))
	Self.mutex.Unlock()
	return
}

func (Self *window) WriteTo() (p []byte, err error) {
	if Self.closeOp {
		logs.Warn("window write to closed")
		return nil, errors.New("conn.writeWindow: window closed")
	}
	if Self.len() == 0 {
		return nil, io.EOF
		// send window buff is drain, return eof and get another one
	}
	var windowSize uint16
	var ok bool
waiting:
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	// waiting for receive usable window size, or timeout
	select {
	case windowSize, ok = <-Self.usableReceiveWindow:
		if !ok {
			return nil, errors.New("conn.writeWindow: window closed")
		}
	case <-ticker.C:
		return nil, errors.New("conn.writeWindow: write to time out")
	}
	if windowSize == 0 {
		goto waiting // waiting for another usable window size
	}
	Self.mutex.Lock()
	if windowSize > uint16(Self.len()) {
		// usable window size is bigger than window buff size, send the full buff
		//logs.Warn("window size overflow windowSize len()", windowSize, Self.len())
		windowSize = uint16(Self.len())
	}
	//logs.Warn("window buff off windowSize", Self.off, windowSize)
	p = Self.windowBuff[Self.off : windowSize+Self.off]
	Self.off += windowSize
	Self.mutex.Unlock()
	return
}

func (Self *window) SetAllowSize(value uint16) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()
	if Self.closeOp {
		close(Self.usableReceiveWindow)
		return true
	}
	select {
	case Self.usableReceiveWindow <- value:
		return false
	case <-Self.closeOpCh:
		close(Self.usableReceiveWindow)
		return true
	}
}

func (Self *window) CloseWindow() {
	Self.closeOp = true
	Self.closeOpCh <- struct{}{}
	Self.closeOpCh <- struct{}{}
	close(Self.closeOpCh)
	return
}
