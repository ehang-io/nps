package nps_mux

import (
	"errors"
	"io"
	"log"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	muxPingFlag uint8 = iota
	muxNewConnOk
	muxNewConnFail
	muxNewMsg
	muxNewMsgPart
	muxMsgSendOk
	muxNewConn
	muxConnClose
	muxPingReturn
	muxPing            int32 = -1
	maximumSegmentSize       = poolSizeWindow
	maximumWindowSize        = 1 << 27 // 1<<31-1 TCP slide window size is very large,
	// we use 128M, reduce memory usage
)

type Mux struct {
	latency uint64 // we store latency in bits, but it's float64
	net.Listener
	conn               net.Conn
	connMap            *connMap
	newConnCh          chan *conn
	done               chan struct{} // closed by Close; newConnCh is not, since a send on it may be waiting
	id                 int32
	closeChan          chan struct{}
	IsClose            bool
	counter            *latencyCounter
	bw                 *bandwidth
	pingCh             chan []byte
	pingCheckTime      uint32 // pings sent and not yet answered; one goes per pingInterval
	pingInterval       time.Duration
	pendingAccept      int32 // streams opened by the peer and not yet accepted; atomic
	pingCheckThreshold uint32
	connType           string
	writeQueue         sendQueue
	newConnQueue       connQueue
	closeOnce          sync.Once
	statsSrc           func() []byte // client side: returns the record to ride on a ping, or nil
	statsSink          func([]byte)  // server side: receives the record a peer's ping carried
	statsAccepted      uint32        // client side: set once the server has said it takes stats
}

// underlying peels wrapping connections, a *tls.Conn above all, off c to get
// at the socket the mux tunes and measures. The bytes are the wrapper's, but
// the queueing happens in the kernel buffers of the socket beneath it.
func underlying(c net.Conn) net.Conn {
	for {
		w, ok := c.(interface{ NetConn() net.Conn })
		if !ok {
			return c
		}
		c = w.NetConn()
	}
}

func NewMux(c net.Conn, connType string, pingCheckThreshold int) *Mux {
	return NewMuxStats(c, connType, pingCheckThreshold, nil, nil)
}

// NewMuxStats is NewMux with a stats record riding on the pings: src, when
// set, supplies the record this side's pings carry (see StatsLen) once the
// peer has said it takes stats, and sink, when set, receives the record of
// every ping the peer sends and tells the peer that it does. Both have to be
// given here because the first ping goes out before NewMux returns.
func NewMuxStats(c net.Conn, connType string, pingCheckThreshold int, src func() []byte, sink func([]byte)) *Mux {
	return newMux(c, connType, pingCheckThreshold, src, sink, defaultPingInterval)
}

// defaultPingInterval is how often a mux pings its peer, and so, times the
// threshold, how long a dead peer goes unnoticed.
const defaultPingInterval = 5 * time.Second

// maxPendingAccept bounds the streams a peer may have opened that the
// application has not yet accepted. Their data is held for them, so without a
// bound a peer could ask a mux whose Accept is stuck to hold any amount.
const maxPendingAccept = 4096

// NewMuxLiveness is NewMux with the interval of the pings given, for a mux
// whose peer's death has to be noticed in seconds, not minutes: the threshold
// times the interval is the time to give a silent peer up.
func NewMuxLiveness(c net.Conn, connType string, pingCheckThreshold int, interval time.Duration) *Mux {
	return newMux(c, connType, pingCheckThreshold, nil, nil, interval)
}

// Healthy reports whether the peer has answered a ping lately: at most one is
// outstanding. A mux that is not is probably dead and not yet known to be, so
// new work is better sent elsewhere, but it is not closed on this account.
func (s *Mux) Healthy() bool {
	return !s.IsClose && atomic.LoadUint32(&s.pingCheckTime) <= 1
}

func newMux(c net.Conn, connType string, pingCheckThreshold int, src func() []byte, sink func([]byte), interval time.Duration) *Mux {
	//c.(*net.TCPConn).SetReadBuffer(0)
	//c.(*net.TCPConn).SetWriteBuffer(0)
	tuneTCP(c)
	raw, err := getRawConn(c)
	if err != nil {
		log.Println(err)
	}
	var checkThreshold uint32
	if pingCheckThreshold <= 0 {
		if connType == "kcp" {
			checkThreshold = 20
		} else {
			checkThreshold = 60
		}
	} else {
		checkThreshold = uint32(pingCheckThreshold)
	}
	m := &Mux{
		conn:               c,
		connMap:            NewConnMap(),
		id:                 0,
		closeChan:          make(chan struct{}, 1),
		newConnCh:          make(chan *conn),
		done:               make(chan struct{}),
		bw:                 NewBandwidth(raw),
		IsClose:            false,
		connType:           connType,
		pingCh:             make(chan []byte),
		pingCheckThreshold: checkThreshold,
		pingInterval:       interval,
		counter:            newLatencyCounter(),
		statsSrc:           src,
		statsSink:          sink,
	}
	m.writeQueue.New()
	m.newConnQueue.New()
	//read session by flag
	m.readSession()
	//ping
	m.ping()
	m.writeSession()
	return m
}

func (s *Mux) NewConn() (*conn, error) {
	conn, err := s.open()
	if err != nil {
		return nil, err
	}
	//Set a timer timeout 120 second
	timer := time.NewTimer(time.Minute * 2)
	defer timer.Stop()
	select {
	case <-conn.connStatusOkCh:
		return conn, nil
	case <-conn.connStatusFailCh:
		_ = conn.Close()
		return nil, errors.New("create connection fail，the server refused the connection")
	case <-conn.closedCh:
		// a closing mux closes every conn in its map, this one included
		return nil, errors.New("create connection fail，the mux has closed")
	case <-timer.C:
	}
	_ = conn.Close()
	return nil, errors.New("create connection fail，the server refused the connection")
}

// NewConnNoWait opens a stream and returns at once, without waiting for the
// peer to accept it, so data can follow the opening frame in the same flight
// and a new stream costs no round trip. The peer's OK is not looked for, and
// a peer that does not accept is found out by its closing the stream.
//
// It needs a peer that registers a stream when the opening frame arrives, as
// this package now does, and not when the stream is accepted: an older peer
// drops data that comes for a stream it has not yet accepted.
func (s *Mux) NewConnNoWait() (*conn, error) {
	return s.open()
}

// open registers a new stream and sends the frame that opens it.
func (s *Mux) open() (*conn, error) {
	if s.IsClose {
		return nil, errors.New("the mux has closed")
	}
	conn := NewConn(s.getId(), s)
	//it must be Set before send
	s.connMap.Set(conn.connId, conn)
	if s.IsClose {
		// closed after the check above: connMap.Close may already have run
		// and would never see this conn
		_ = conn.Close()
		return nil, errors.New("the mux has closed")
	}
	s.sendInfo(muxNewConn, conn.connId, nil)
	return conn, nil
}

func (s *Mux) Accept() (net.Conn, error) {
	if s.IsClose {
		return nil, errors.New("accpet error,the mux has closed")
	}
	select {
	case conn := <-s.newConnCh:
		return conn, nil
	case <-s.done:
		return nil, errors.New("accpet error,the mux has closed")
	}
}

func (s *Mux) Addr() net.Addr {
	return s.conn.LocalAddr()
}

// sendInfo queues a frame. The channel it may return is the write queue's
// backpressure for stream data, see sendQueue.Push.
func (s *Mux) sendInfo(flag uint8, id int32, data interface{}) (drained <-chan struct{}) {
	if s.IsClose {
		return
	}
	var err error
	pack := muxPack.Get()
	err = pack.Set(flag, id, data)
	if err != nil {
		muxPack.Put(pack)
		log.Println("mux: New Pack err", err)
		_ = s.Close()
		return
	}
	return s.writeQueue.Push(pack)
}

// writeBatchSize caps how many bytes of frames go to the connection in one
// write. Once written their order is fixed, so this is also how long a newly
// queued frame from another stream may have to wait.
const writeBatchSize = 16 * 1024

func (s *Mux) writeSession() {
	go func() {
		packs := make([]*muxPackager, 0, 16)
		vec := make(net.Buffers, 0, 32)
		for {
			if s.IsClose {
				break
			}
			pack := s.writeQueue.Pop()
			if s.IsClose {
				break
			}
			bufs, size := vec[:0], 0
			for pack != nil {
				packs = append(packs, pack)
				bufs = pack.appendTo(bufs)
				size += pack.wireSize()
				if size >= writeBatchSize {
					break
				}
				pack = s.writeQueue.TryPop()
			}
			vec = bufs[:0]
			_, err := bufs.WriteTo(s.conn)
			for i, p := range packs {
				p.free()
				muxPack.Put(p)
				packs[i] = nil
			}
			packs = packs[:0]
			if err != nil {
				log.Println("mux: Pack err", err)
				_ = s.Close()
				break
			}
		}
	}()
}

// sendPing sends one ping: the timestamp the peer echoes back for the
// latency, behind the stats record when there is one to send.
func (s *Mux) sendPing() {
	now, _ := time.Now().UTC().MarshalText()
	switch {
	case s.statsSink != nil:
		now = append([]byte{StatsAccept}, now...)
	case s.statsSrc != nil && atomic.LoadUint32(&s.statsAccepted) == 1:
		if rec := s.statsSrc(); len(rec) == StatsLen {
			now = append(append(make([]byte, 0, StatsLen+len(now)), rec...), now...)
		}
	}
	s.sendInfo(muxPingFlag, muxPing, now)
}

func (s *Mux) ping() {
	go func() {
		s.sendPing()
		// send the ping flag and Get the latency first
		ticker := time.NewTicker(s.pingInterval)
		defer ticker.Stop()
		for {
			if s.IsClose {
				break
			}
			select {
			case <-ticker.C:
			}
			if atomic.LoadUint32(&s.pingCheckTime) > s.pingCheckThreshold {
				log.Println("mux: ping time out, checktime", s.pingCheckTime, "threshold", s.pingCheckThreshold)
				_ = s.Close()
				// more than limit times not receive the ping return package,
				// mux conn is damaged, maybe a packet drop, close it
				break
			}
			s.sendPing()
			atomic.AddUint32(&s.pingCheckTime, 1)
		}
		return
	}()

	go func() {
		var now time.Time
		var data []byte
		for {
			if s.IsClose {
				break
			}
			select {
			case data = <-s.pingCh:
				atomic.StoreUint32(&s.pingCheckTime, 0)
			case <-s.closeChan:
				// nothing arrived: data is the last ping, already given back
				return
			}
			_ = now.UnmarshalText(stripStats(data))
			latency := time.Now().UTC().Sub(now).Seconds()
			if latency > 0 {
				atomic.StoreUint64(&s.latency, math.Float64bits(s.counter.Latency(latency)))
				// convert float64 to bits, store it atomic
				//log.Println("ping", math.Float64frombits(atomic.LoadUint64(&s.latency)))
			}
			if cap(data) > 0 && !s.IsClose {
				windowBuff.Put(data)
			}
		}
	}()
}

func (s *Mux) readSession() {
	go func() {
		var connection *conn
		for {
			if s.IsClose {
				break
			}
			connection = s.newConnQueue.Pop()
			if s.IsClose {
				break // make sure that is closed
			}
			atomic.AddInt32(&s.pendingAccept, -1)
			select {
			case s.newConnCh <- connection:
			case <-s.done:
				return
			}
			s.sendInfo(muxNewConnOk, connection.connId, nil)
		}
	}()
	go func() {
		var pack *muxPackager
		var l uint16
		var err error
		for {
			if s.IsClose {
				return
			}
			pack = muxPack.Get()
			s.bw.StartRead()
			if l, err = pack.UnPack(s.conn); err != nil {
				log.Println("mux: read session unpack from connection err", err)
				_ = s.Close()
				break
			}
			s.bw.SetCopySize(l)
			//if pack.flag == muxNewMsg || pack.flag == muxNewMsgPart {
			//	if pack.length >= 100 {
			//		log.Printf("read session id %d pointer %p\n%v", pack.id, pack.content, string(pack.content[:100]))
			//	} else {
			//		log.Printf("read session id %d pointer %p\n%v", pack.id, pack.content, string(pack.content[:pack.length]))
			//	}
			//}
			switch pack.flag {
			case muxNewConn: //New connection
				if atomic.AddInt32(&s.pendingAccept, 1) > maxPendingAccept {
					// not taking more than this on trust: refuse the stream
					atomic.AddInt32(&s.pendingAccept, -1)
					s.sendInfo(muxConnClose, pack.id, nil)
					continue
				}
				connection := NewConn(pack.id, s)
				// Registered now, not when it is accepted: a peer that does
				// not wait for the OK sends data right behind this frame,
				// and a frame for a stream that is not in the map is dropped.
				s.connMap.Set(connection.connId, connection)
				s.newConnQueue.Push(connection)
				continue
			case muxPingFlag: //ping
				if s.statsSink != nil && hasStats(pack.content) {
					s.statsSink(append([]byte(nil), pack.content[:StatsLen]...))
				}
				if s.statsSrc != nil && hasAccept(pack.content) &&
					atomic.CompareAndSwapUint32(&s.statsAccepted, 0, 1) {
					// the first ping went out before the server had spoken;
					// send the record now instead of at the next tick
					s.sendPing()
				}
				s.sendInfo(muxPingReturn, muxPing, pack.content)
				windowBuff.Put(pack.content)
				continue
			case muxPingReturn:
				s.pingCh <- pack.content
				continue
			}
			if connection, ok := s.connMap.Get(pack.id); ok && !connection.isClose {
				switch pack.flag {
				case muxNewMsg, muxNewMsgPart: //New msg from remote connection
					err = s.newMsg(connection, pack)
					if err != nil {
						log.Println("mux: read session connection New msg err", err)
						_ = connection.Close()
					}
					continue
				case muxNewConnOk: //connection ok
					select {
					case connection.connStatusOkCh <- struct{}{}:
					default:
					}
					continue
				case muxNewConnFail:
					select {
					case connection.connStatusFailCh <- struct{}{}:
					default:
					}
					continue
				case muxMsgSendOk:
					if connection.isClose {
						continue
					}
					connection.sendWindow.SetSize(pack.window)
					continue
				case muxConnClose: //close the connection
					connection.closingFlag = true
					connection.receiveWindow.Stop() // close signal to receive window
					continue
				}
			} else if pack.flag == muxConnClose {
				continue
			}
			muxPack.Put(pack)
		}
	}()
}

func (s *Mux) newMsg(connection *conn, pack *muxPackager) (err error) {
	if connection.isClose {
		err = io.ErrClosedPipe
		return
	}
	//insert into queue
	if pack.flag == muxNewMsgPart {
		err = connection.receiveWindow.Write(pack.content, pack.length, true, pack.id)
	}
	if pack.flag == muxNewMsg {
		err = connection.receiveWindow.Write(pack.content, pack.length, false, pack.id)
	}
	return
}

func (s *Mux) Close() (err error) {
	err = errors.New("the mux has closed")
	// Once, not just the IsClose check: the bridge closes a client's muxes
	// while their own read and write loops may be closing them on an error,
	// and a second pass would close newConnCh again and panic.
	s.closeOnce.Do(func() {
		err = nil
		s.IsClose = true
		log.Println("close mux")
		s.connMap.Close()
		//s.connMap = nil
		s.closeChan <- struct{}{}
		close(s.done)
		// while target host close socket without finish steps, conn.Close method maybe blocked
		// and tcp status change to CLOSE WAIT or TIME WAIT, so we close it in other goroutine
		_ = s.conn.SetDeadline(time.Now().Add(time.Second * 5))
		go s.conn.Close()
		s.release()
	})
	return
}

func (s *Mux) release() {
	for {
		pack := s.writeQueue.TryPop()
		if pack == nil {
			break
		}
		pack.free()
		muxPack.Put(pack)
	}
	for {
		connection := s.newConnQueue.TryPop()
		if connection == nil {
			break
		}
		connection = nil
	}
	s.writeQueue.Stop()
	s.newConnQueue.Stop()
}

//Get New connId as unique flag
func (s *Mux) getId() (id int32) {
	//Avoid going beyond the scope
	if (math.MaxInt32 - s.id) < 10000 {
		atomic.StoreInt32(&s.id, 0)
	}
	id = atomic.AddInt32(&s.id, 1)
	if _, ok := s.connMap.Get(id); ok {
		return s.getId()
	}
	return
}

type bandwidth struct {
	readBandwidth uint64 // store in bits, but it's float64
	readStart     time.Time
	lastReadStart time.Time
	bufLength     uint32
	raw           syscall.RawConn
	calcThreshold uint32
}

func NewBandwidth(raw syscall.RawConn) *bandwidth {
	return &bandwidth{raw: raw}
}

func (Self *bandwidth) StartRead() {
	if Self.readStart.IsZero() {
		Self.readStart = time.Now()
	}
	if Self.bufLength >= Self.calcThreshold {
		Self.lastReadStart, Self.readStart = Self.readStart, time.Now()
		Self.calcBandWidth()
	}
}

func (Self *bandwidth) SetCopySize(n uint16) {
	Self.bufLength += uint32(n)
}

func (Self *bandwidth) calcBandWidth() {
	t := Self.readStart.Sub(Self.lastReadStart)
	bufferSize, err := sysGetSock(Self.raw)
	if err != nil {
		log.Println(err)
		Self.bufLength = 0
		return
	}
	if Self.bufLength >= uint32(bufferSize) {
		atomic.StoreUint64(&Self.readBandwidth, math.Float64bits(float64(Self.bufLength)/t.Seconds()))
		// calculate the whole socket buffer, the time meaning to fill the buffer
	} else {
		Self.calcThreshold = uint32(bufferSize)
	}
	// socket buffer size is bigger than bufLength, so we don't calculate it
	Self.bufLength = 0
}

func (Self *bandwidth) Get() (bw float64) {
	// The zero value, 0 for numeric types
	bw = math.Float64frombits(atomic.LoadUint64(&Self.readBandwidth))
	if bw <= 0 {
		bw = 0
	}
	return
}

const counterBits = 4
const counterMask = 1<<counterBits - 1

func newLatencyCounter() *latencyCounter {
	return &latencyCounter{
		buf:     make([]float64, 1<<counterBits, 1<<counterBits),
		headMin: 0,
	}
}

type latencyCounter struct {
	buf []float64 //buf is a fixed length ring buffer,
	// if buffer is full, New value will replace the oldest one.
	headMin uint8 //head indicate the head in ring buffer,
	// in meaning, slot in list will be replaced;
	// min indicate this slot value is minimal in list.

	// we delineate the effective range with three times the minimum latency
	// average of effective latency for all current data as a mux latency
}

func (Self *latencyCounter) unpack(idxs uint8) (head, min uint8) {
	head = (idxs >> counterBits) & counterMask
	// we Set head is 4 bits
	min = idxs & counterMask
	return
}

func (Self *latencyCounter) pack(head, min uint8) uint8 {
	return head<<counterBits |
		min&counterMask
}

func (Self *latencyCounter) add(value float64) {
	head, min := Self.unpack(Self.headMin)
	Self.buf[head] = value
	if head == min {
		min = Self.minimal()
		//if head equals min, means the min slot already be replaced,
		// so we need to find another minimal value in the list,
		// and change the min indicator
	}
	if Self.buf[min] > value {
		min = head
	}
	head++
	Self.headMin = Self.pack(head, min)
}

func (Self *latencyCounter) minimal() (min uint8) {
	var val float64
	var i uint8
	for i = 0; i < counterMask; i++ {
		if Self.buf[i] > 0 {
			if val > Self.buf[i] {
				val = Self.buf[i]
				min = i
			}
		}
	}
	return
}

func (Self *latencyCounter) Latency(value float64) (latency float64) {
	Self.add(value)
	latency = Self.countSuccess()
	return
}

const lossRatio = 3

func (Self *latencyCounter) countSuccess() (successRate float64) {
	var i, success uint8
	_, min := Self.unpack(Self.headMin)
	for i = 0; i < counterMask; i++ {
		if Self.buf[i] <= lossRatio*Self.buf[min] && Self.buf[i] > 0 {
			success++
			successRate += Self.buf[i]
		}
	}
	// counting all the data in the ring buf, except zero
	successRate = successRate / float64(success)
	return
}
