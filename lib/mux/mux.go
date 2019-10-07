package mux

import (
	"bytes"
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
	pingCh     chan []byte
	connType   string
	writeQueue PriorityQueue
	bufCh      chan *bytes.Buffer
	sync.Mutex
}

func NewMux(c net.Conn, connType string) *Mux {
	m := &Mux{
		conn:      c,
		connMap:   NewConnMap(),
		id:        0,
		closeChan: make(chan struct{}),
		newConnCh: make(chan *conn),
		IsClose:   false,
		connType:  connType,
		bufCh:     make(chan *bytes.Buffer),
		pingCh:    make(chan []byte),
	}
	m.writeQueue.New()
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
	conn := NewConn(s.getId(), s)
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
	go s.writeBuf()
}

func (s *Mux) packBuf() {
	for {
		if s.IsClose {
			break
		}
		pack := s.writeQueue.Pop()
		buffer := common.BuffPool.Get()
		err := pack.Pack(buffer)
		common.MuxPack.Put(pack)
		if err != nil {
			logs.Warn("pack err", err)
			common.BuffPool.Put(buffer)
			break
		}
		select {
		case s.bufCh <- buffer:
		case <-s.closeChan:
			break
		}
	}
}

func (s *Mux) writeBuf() {
	for {
		if s.IsClose {
			break
		}
		select {
		case buffer := <-s.bufCh:
			l := buffer.Len()
			n, err := buffer.WriteTo(s.conn)
			common.BuffPool.Put(buffer)
			if err != nil || int(n) != l {
				logs.Warn("close from write session fail ", err, n, l)
				s.Close()
				break
			}
		case <-s.closeChan:
			break
		}
	}
}

func (s *Mux) ping() {
	go func() {
		now, _ := time.Now().MarshalText()
		s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, now)
		// send the ping flag and get the latency first
		ticker := time.NewTicker(time.Second * 15)
		for {
			if s.IsClose {
				ticker.Stop()
				break
			}
			select {
			case <-ticker.C:
			}
			//Avoid going beyond the scope
			if (math.MaxInt32 - s.id) < 10000 {
				s.id = 0
			}
			now, _ := time.Now().MarshalText()
			s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, now)
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
			select {
			case data = <-s.pingCh:
			case <-s.closeChan:
				break
			}
			_ = now.UnmarshalText(data)
			s.latency = time.Since(now).Seconds()
			s.sendInfo(common.MUX_PING_RETURN, common.MUX_PING, nil)
		}
	}()
}

func (s *Mux) readSession() {
	go func() {
		pack := common.MuxPack.Get()
		for {
			if s.IsClose {
				break
			}
			pack = common.MuxPack.Get()
			if pack.UnPack(s.conn) != nil {
				break
			}
			s.pingOk = 0
			switch pack.Flag {
			case common.MUX_NEW_CONN: //new connection
				connection := NewConn(pack.Id, s)
				s.connMap.Set(pack.Id, connection) //it has been set before send ok
				s.newConnCh <- connection
				s.sendInfo(common.MUX_NEW_CONN_OK, connection.connId, nil)
				continue
			case common.MUX_PING_FLAG: //ping
				s.pingCh <- pack.Content
				continue
			case common.MUX_PING_RETURN:
				continue
			}
			if connection, ok := s.connMap.Get(pack.Id); ok && !connection.isClose {
				switch pack.Flag {
				case common.MUX_NEW_MSG, common.MUX_NEW_MSG_PART: //new msg from remote connection
					err := s.newMsg(connection, pack)
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
	s.closeChan <- struct{}{}
	s.closeChan <- struct{}{}
	close(s.newConnCh)
	return s.conn.Close()
}

//get new connId as unique flag
func (s *Mux) getId() (id int32) {
	id = atomic.AddInt32(&s.id, 1)
	if _, ok := s.connMap.Get(id); ok {
		s.getId()
	}
	return
}
