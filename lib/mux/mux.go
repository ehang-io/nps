package mux

import (
	"errors"
	"io"
	"math"
	"net"
	"sync/atomic"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/lib/common"
)

type Mux struct {
	latency uint64 // we store latency in bits, but it's float64
	net.Listener
	conn          net.Conn
	connMap       *connMap
	newConnCh     chan *conn
	id            int32
	closeChan     chan struct{}
	IsClose       bool
	pingOk        uint32
	counter       *latencyCounter
	bw            *bandwidth
	pingCh        chan []byte
	pingCheckTime uint32
	connType      string
	writeQueue    PriorityQueue
	newConnQueue  ConnQueue
}

func NewMux(c net.Conn, connType string) *Mux {
	//c.(*net.TCPConn).SetReadBuffer(0)
	//c.(*net.TCPConn).SetWriteBuffer(0)
	m := &Mux{
		conn:      c,
		connMap:   NewConnMap(),
		id:        0,
		closeChan: make(chan struct{}, 1),
		newConnCh: make(chan *conn),
		bw:        new(bandwidth),
		IsClose:   false,
		connType:  connType,
		pingCh:    make(chan []byte),
		counter:   newLatencyCounter(),
	}
	m.writeQueue.New()
	m.newConnQueue.New()
	//read session by flag
	m.readSession()
	//ping
	m.ping()
	m.pingReturn()
	m.writeSession()
	return m
}

func (s *Mux) NewConn() (*conn, error) {
	if s.IsClose {
		return nil, errors.New("the mux has closed")
	}
	conn := NewConn(s.getId(), s, "nps ")
	//it must be set before send
	s.connMap.Set(conn.connId, conn)
	s.sendInfo(common.MUX_NEW_CONN, conn.connId, nil)
	//set a timer timeout 30 second
	timer := time.NewTimer(time.Minute * 2)
	defer timer.Stop()
	select {
	case <-conn.connStatusOkCh:
		return conn, nil
	case <-conn.connStatusFailCh:
	case <-timer.C:
	}
	return nil, errors.New("create connection fail，the server refused the connection")
}

func (s *Mux) Accept() (net.Conn, error) {
	if s.IsClose {
		return nil, errors.New("accpet error,the mux has closed")
	}
	conn := <-s.newConnCh
	if conn == nil {
		return nil, errors.New("accpet error,the conn has closed")
	}
	return conn, nil
}

func (s *Mux) Addr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Mux) sendInfo(flag uint8, id int32, data ...interface{}) {
	if s.IsClose {
		return
	}
	var err error
	pack := common.MuxPack.Get()
	err = pack.NewPac(flag, id, data...)
	if err != nil {
		common.MuxPack.Put(pack)
		logs.Error("mux: new pack err", err)
		s.Close()
		return
	}
	s.writeQueue.Push(pack)
	return
}

func (s *Mux) writeSession() {
	go s.packBuf()
	//go s.writeBuf()
}

func (s *Mux) packBuf() {
	//buffer := common.BuffPool.Get()
	for {
		if s.IsClose {
			break
		}
		//buffer.Reset()
		pack := s.writeQueue.Pop()
		if s.IsClose {
			break
		}
		//buffer := common.BuffPool.Get()
		err := pack.Pack(s.conn)
		common.MuxPack.Put(pack)
		if err != nil {
			logs.Error("mux: pack err", err)
			//common.BuffPool.Put(buffer)
			s.Close()
			break
		}
		//logs.Warn(buffer.String())
		//s.bufQueue.Push(buffer)
		//l := buffer.Len()
		//n, err := buffer.WriteTo(s.conn)
		//common.BuffPool.Put(buffer)
		//if err != nil || int(n) != l {
		//	logs.Error("mux: close from write session fail ", err, n, l)
		//	s.Close()
		//	break
		//}
	}
}

//func (s *Mux) writeBuf() {
//	for {
//		if s.IsClose {
//			break
//		}
//		buffer, err := s.bufQueue.Pop()
//		if err != nil {
//			break
//		}
//		l := buffer.Len()
//		n, err := buffer.WriteTo(s.conn)
//		common.BuffPool.Put(buffer)
//		if err != nil || int(n) != l {
//			logs.Warn("close from write session fail ", err, n, l)
//			s.Close()
//			break
//		}
//	}
//}

func (s *Mux) ping() {
	go func() {
		now, _ := time.Now().UTC().MarshalText()
		s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, now)
		// send the ping flag and get the latency first
		ticker := time.NewTicker(time.Second * 5)
    defer ticker.Stop()
		for {
			if s.IsClose {
				break
			}
			select {
			case <-ticker.C:
			}
			if atomic.LoadUint32(&s.pingCheckTime) >= 60 {
				logs.Error("mux: ping time out")
				s.Close()
				// more than 5 minutes not receive the ping return package,
				// mux conn is damaged, maybe a packet drop, close it
				break
			}
			now, _ := time.Now().UTC().MarshalText()
			s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, now)
			atomic.AddUint32(&s.pingCheckTime, 1)
			if atomic.LoadUint32(&s.pingOk) > 10 && s.connType == "kcp" {
				logs.Error("mux: kcp ping err")
				s.Close()
				break
			}
			atomic.AddUint32(&s.pingOk, 1)
		}
    return
	}()
}

func (s *Mux) pingReturn() {
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
				break
			}
			_ = now.UnmarshalText(data)
			latency := time.Now().UTC().Sub(now).Seconds() / 2
			if latency > 0 {
				atomic.StoreUint64(&s.latency, math.Float64bits(s.counter.Latency(latency)))
				// convert float64 to bits, store it atomic
			}
			//logs.Warn("latency", math.Float64frombits(atomic.LoadUint64(&s.latency)))
			if cap(data) > 0 {
				common.WindowBuff.Put(data)
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
			s.connMap.Set(connection.connId, connection) //it has been set before send ok
			s.newConnCh <- connection
			s.sendInfo(common.MUX_NEW_CONN_OK, connection.connId, nil)
		}
	}()
	go func() {
		pack := common.MuxPack.Get()
		var l uint16
		var err error
		for {
			if s.IsClose {
				break
			}
			pack = common.MuxPack.Get()
			s.bw.StartRead()
			if l, err = pack.UnPack(s.conn); err != nil {
				logs.Error("mux: read session unpack from connection err", err)
				s.Close()
				break
			}
			s.bw.SetCopySize(l)
			atomic.StoreUint32(&s.pingOk, 0)
			switch pack.Flag {
			case common.MUX_NEW_CONN: //new connection
				connection := NewConn(pack.Id, s)
				s.newConnQueue.Push(connection)
				continue
			case common.MUX_PING_FLAG: //ping
				s.sendInfo(common.MUX_PING_RETURN, common.MUX_PING, pack.Content)
				common.WindowBuff.Put(pack.Content)
				continue
			case common.MUX_PING_RETURN:
				//go func(content []byte) {
				s.pingCh <- pack.Content
				//}(pack.Content)
				continue
			}
			if connection, ok := s.connMap.Get(pack.Id); ok && !connection.isClose {
				switch pack.Flag {
				case common.MUX_NEW_MSG, common.MUX_NEW_MSG_PART: //new msg from remote connection
					err = s.newMsg(connection, pack)
					if err != nil {
						logs.Error("mux: read session connection new msg err", err)
						connection.Close()
					}
					continue
				case common.MUX_NEW_CONN_OK: //connection ok
					connection.connStatusOkCh <- struct{}{}
					continue
				case common.MUX_NEW_CONN_Fail:
					connection.connStatusFailCh <- struct{}{}
					continue
				case common.MUX_MSG_SEND_OK:
					if connection.isClose {
						continue
					}
					connection.sendWindow.SetSize(pack.Window, pack.ReadLength)
					continue
				case common.MUX_CONN_CLOSE: //close the connection
					connection.closeFlag = true
					//s.connMap.Delete(pack.Id)
					//go func(connection *conn) {
					connection.receiveWindow.Stop() // close signal to receive window
					//}(connection)
					continue
				}
			} else if pack.Flag == common.MUX_CONN_CLOSE {
				continue
			}
			common.MuxPack.Put(pack)
		}
		common.MuxPack.Put(pack)
		s.Close()
	}()
}

func (s *Mux) newMsg(connection *conn, pack *common.MuxPackager) (err error) {
	if connection.isClose {
		err = io.ErrClosedPipe
		return
	}
	//logs.Warn("read session receive new msg", pack.Length)
	//go func(connection *conn, pack *common.MuxPackager) { // do not block read session
	//insert into queue
	if pack.Flag == common.MUX_NEW_MSG_PART {
		err = connection.receiveWindow.Write(pack.Content, pack.Length, true, pack.Id)
	}
	if pack.Flag == common.MUX_NEW_MSG {
		err = connection.receiveWindow.Write(pack.Content, pack.Length, false, pack.Id)
	}
	//logs.Warn("read session write success", pack.Length)
	return
}

func (s *Mux) Close() (err error) {
	logs.Warn("close mux")
	if s.IsClose {
		return errors.New("the mux has closed")
	}
	s.IsClose = true
	s.connMap.Close()
	s.connMap = nil
	//s.bufQueue.Stop()
	s.closeChan <- struct{}{}
	close(s.newConnCh)
	err = s.conn.Close()
	s.release()
	return
}

func (s *Mux) release() {
	for {
		pack := s.writeQueue.TryPop()
		if pack == nil {
			break
		}
		if pack.BasePackager.Content != nil {
			common.WindowBuff.Put(pack.BasePackager.Content)
		}
		common.MuxPack.Put(pack)
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

//get new connId as unique flag
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
}

func (Self *bandwidth) StartRead() {
	if Self.readStart.IsZero() {
		Self.readStart = time.Now()
	}
	if Self.bufLength >= common.MAXIMUM_SEGMENT_SIZE*300 {
		Self.lastReadStart, Self.readStart = Self.readStart, time.Now()
		Self.calcBandWidth()
	}
}

func (Self *bandwidth) SetCopySize(n uint16) {
	Self.bufLength += uint32(n)
}

func (Self *bandwidth) calcBandWidth() {
	t := Self.readStart.Sub(Self.lastReadStart)
	atomic.StoreUint64(&Self.readBandwidth, math.Float64bits(float64(Self.bufLength)/t.Seconds()))
	Self.bufLength = 0
}

func (Self *bandwidth) Get() (bw float64) {
	// The zero value, 0 for numeric types
	bw = math.Float64frombits(atomic.LoadUint64(&Self.readBandwidth))
	if bw <= 0 {
		bw = 100
	}
	//logs.Warn(bw)
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
	// if buffer is full, new value will replace the oldest one.
	headMin uint8 //head indicate the head in ring buffer,
	// in meaning, slot in list will be replaced;
	// min indicate this slot value is minimal in list.
}

func (Self *latencyCounter) unpack(idxs uint8) (head, min uint8) {
	head = uint8((idxs >> counterBits) & counterMask)
	// we set head is 4 bits
	min = uint8(idxs & counterMask)
	return
}

func (Self *latencyCounter) pack(head, min uint8) uint8 {
	return uint8(head<<counterBits) |
		uint8(min&counterMask)
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
	_, min := Self.unpack(Self.headMin)
	latency = Self.buf[min] * Self.countSuccess()
	return
}

const lossRatio = 1.6

func (Self *latencyCounter) countSuccess() (successRate float64) {
	var success, loss, i uint8
	_, min := Self.unpack(Self.headMin)
	for i = 0; i < counterMask; i++ {
		if Self.buf[i] > lossRatio*Self.buf[min] && Self.buf[i] > 0 {
			loss++
		}
		if Self.buf[i] <= lossRatio*Self.buf[min] && Self.buf[i] > 0 {
			success++
		}
	}
	// counting all the data in the ring buf, except zero
	successRate = float64(success) / float64(loss+success)
	return
}
