package mux

import (
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/lib/common"
)

type Mux struct {
	net.Listener
	conn       net.Conn
	connMap    *connMap
	newConnCh  chan *conn
	id         int32
	closeChan  chan struct{}
	IsClose    bool
	pingOk     int
	latency    float64
	bw         *bandwidth
	pingCh     chan []byte
	pingTimer  *time.Timer
	connType   string
	writeQueue PriorityQueue
	//bufQueue      BytesQueue
	sync.Mutex
}

func NewMux(c net.Conn, connType string) *Mux {
	//c.(*net.TCPConn).SetReadBuffer(0)
	//c.(*net.TCPConn).SetWriteBuffer(0)
	m := &Mux{
		conn:      c,
		connMap:   NewConnMap(),
		id:        0,
		closeChan: make(chan struct{}, 1),
		newConnCh: make(chan *conn, 10),
		bw:        new(bandwidth),
		IsClose:   false,
		connType:  connType,
		pingCh:    make(chan []byte),
		pingTimer: time.NewTimer(15 * time.Second),
	}
	m.writeQueue.New()
	//m.bufQueue.New()
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
	buffer := common.BuffPool.Get()
	for {
		if s.IsClose {
			break
		}
		buffer.Reset()
		pack := s.writeQueue.Pop()
		//buffer := common.BuffPool.Get()
		err := pack.Pack(buffer)
		common.MuxPack.Put(pack)
		if err != nil {
			logs.Warn("pack err", err)
			common.BuffPool.Put(buffer)
			break
		}
		//logs.Warn(buffer.String())
		//s.bufQueue.Push(buffer)
		l := buffer.Len()
		n, err := buffer.WriteTo(s.conn)
		//common.BuffPool.Put(buffer)
		if err != nil || int(n) != l {
			logs.Warn("close from write session fail ", err, n, l)
			s.Close()
			break
		}
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
		for {
			if s.IsClose {
				ticker.Stop()
				if !s.pingTimer.Stop() {
					<-s.pingTimer.C
				}
				break
			}
			select {
			case <-ticker.C:
			}
			now, _ := time.Now().UTC().MarshalText()
			s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, now)
			if !s.pingTimer.Stop() {
				<-s.pingTimer.C
			}
			s.pingTimer.Reset(15 * time.Second)
			if s.pingOk > 10 && s.connType == "kcp" {
				s.Close()
				break
			}
			s.pingOk++
		}
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
			case <-s.closeChan:
				break
			case <-s.pingTimer.C:
				logs.Error("mux: ping time out")
				s.Close()
				break
			}
			_ = now.UnmarshalText(data)
			latency := time.Now().UTC().Sub(now).Seconds() / 2
			if latency < 0.5 && latency > 0 {
				s.latency = latency
			}
			//logs.Warn("latency", s.latency)
			common.WindowBuff.Put(data)
		}
	}()
}

func (s *Mux) readSession() {
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
				break
			}
			s.bw.SetCopySize(l)
			s.pingOk = 0
			switch pack.Flag {
			case common.MUX_NEW_CONN: //new connection
				connection := NewConn(pack.Id, s, "npc ")
				s.connMap.Set(pack.Id, connection) //it has been set before send ok
				s.newConnCh <- connection
				s.sendInfo(common.MUX_NEW_CONN_OK, connection.connId, nil)
				continue
			case common.MUX_PING_FLAG: //ping
				s.sendInfo(common.MUX_PING_RETURN, common.MUX_PING, pack.Content)
				common.WindowBuff.Put(pack.Content)
				continue
			case common.MUX_PING_RETURN:
				s.pingCh <- pack.Content
				continue
			}
			if connection, ok := s.connMap.Get(pack.Id); ok && !connection.isClose {
				switch pack.Flag {
				case common.MUX_NEW_MSG, common.MUX_NEW_MSG_PART: //new msg from remote connection
					err = s.newMsg(connection, pack)
					if err != nil {
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
					s.connMap.Delete(pack.Id)
					connection.closeFlag = true
					connection.receiveWindow.Stop() // close signal to receive window
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

func (s *Mux) Close() error {
	logs.Warn("close mux")
	if s.IsClose {
		return errors.New("the mux has closed")
	}
	s.IsClose = true
	s.connMap.Close()
	//s.bufQueue.Stop()
	s.closeChan <- struct{}{}
	close(s.newConnCh)
	return s.conn.Close()
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
	readStart     time.Time
	lastReadStart time.Time
	bufLength     uint16
	readBandwidth float64
}

func (Self *bandwidth) StartRead() {
	if Self.readStart.IsZero() {
		Self.readStart = time.Now()
	}
	if Self.bufLength >= 16384 {
		Self.lastReadStart, Self.readStart = Self.readStart, time.Now()
		Self.calcBandWidth()
	}
}

func (Self *bandwidth) SetCopySize(n uint16) {
	Self.bufLength += n
}

func (Self *bandwidth) calcBandWidth() {
	t := Self.readStart.Sub(Self.lastReadStart)
	Self.readBandwidth = float64(Self.bufLength) / t.Seconds()
	Self.bufLength = 0
}

func (Self *bandwidth) Get() (bw float64) {
	// The zero value, 0 for numeric types
	if Self.readBandwidth <= 0 {
		Self.readBandwidth = 100
	}
	return Self.readBandwidth
}
