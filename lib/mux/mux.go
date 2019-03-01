package mux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/cnlh/nps/lib/pool"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MUX_PING_FLAG int32 = iota
	MUX_NEW_CONN_OK
	MUX_NEW_CONN_Fail
	MUX_NEW_MSG
	MUX_MSG_SEND_OK
	MUX_NEW_CONN
	MUX_PING
	MUX_CONN_CLOSE
)

type Mux struct {
	net.Listener
	conn         net.Conn
	connMap      *connMap
	sendMsgCh    chan *msg  //write msg chan
	sendStatusCh chan int32 //write read ok chan
	newConnCh    chan *conn
	id           int32
	closeChan    chan struct{}
	isClose      bool
	sync.Mutex
}

func NewMux(c net.Conn) *Mux {
	m := &Mux{
		conn:         c,
		connMap:      NewConnMap(),
		sendMsgCh:    make(chan *msg),
		sendStatusCh: make(chan int32),
		id:           0,
		closeChan:    make(chan struct{}),
		newConnCh:    make(chan *conn),
		isClose:      false,
	}
	//read session by flag
	go m.readSession()
	//write session
	go m.writeSession()
	//ping
	go m.ping()
	return m
}

func (s *Mux) NewConn() (*conn, error) {
	if s.isClose {
		return nil, errors.New("the mux has closed")
	}
	conn := NewConn(s.getId(), s, s.sendMsgCh, s.sendStatusCh)
	raw := bytes.NewBuffer([]byte{})
	if err := binary.Write(raw, binary.LittleEndian, MUX_NEW_CONN); err != nil {
		return nil, err
	}
	if err := binary.Write(raw, binary.LittleEndian, conn.connId); err != nil {
		return nil, err
	}
	//it must be set before send
	s.connMap.Set(conn.connId, conn)
	if _, err := s.conn.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	select {
	case <-conn.connStatusOkCh:
		return conn, nil
	case <-conn.connStatusFailCh:
	}
	return nil, errors.New("create connection fail，the server refused the connection")
}

func (s *Mux) Accept() (net.Conn, error) {
	if s.isClose {
		return nil, errors.New("accpet error,the conn has closed")
	}
	return <-s.newConnCh, nil
}

func (s *Mux) Addr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Mux) ping() {
	go func() {
		ticker := time.NewTicker(time.Second * 5)
		raw := bytes.NewBuffer([]byte{})
		for {
			select {
			case <-ticker.C:
			}
			//Avoid going beyond the scope
			if (math.MaxInt32 - s.id) < 10000 {
				s.id = 0
			}
			raw.Reset()
			binary.Write(raw, binary.LittleEndian, MUX_PING_FLAG)
			binary.Write(raw, binary.LittleEndian, MUX_PING)
			if _, err := s.conn.Write(raw.Bytes()); err != nil {
				s.Close()
				break
			}
		}
	}()
	select {
	case <-s.closeChan:
	}
}

func (s *Mux) writeSession() {
	go func() {
		raw := bytes.NewBuffer([]byte{})
		for {
			raw.Reset()
			select {
			case msg := <-s.sendMsgCh:
				if msg == nil {
					break
				}
				if msg.content == nil { //close
					binary.Write(raw, binary.LittleEndian, MUX_CONN_CLOSE)
					binary.Write(raw, binary.LittleEndian, msg.connId)
					break
				}
				binary.Write(raw, binary.LittleEndian, MUX_NEW_MSG)
				binary.Write(raw, binary.LittleEndian, msg.connId)
				binary.Write(raw, binary.LittleEndian, int32(len(msg.content)))
				binary.Write(raw, binary.LittleEndian, msg.content)
			case connId := <-s.sendStatusCh:
				binary.Write(raw, binary.LittleEndian, MUX_MSG_SEND_OK)
				binary.Write(raw, binary.LittleEndian, connId)
			}
			if _, err := s.conn.Write(raw.Bytes()); err != nil {
				s.Close()
				break
			}
		}
	}()
	select {
	case <-s.closeChan:
	}
}

func (s *Mux) readSession() {
	go func() {
		raw := bytes.NewBuffer([]byte{})
		buf := pool.BufPoolCopy.Get().([]byte)
		defer pool.PutBufPoolCopy(buf)
		for {
			var flag, i int32
			var n int
			var err error
			if binary.Read(s.conn, binary.LittleEndian, &flag) == nil {
				if binary.Read(s.conn, binary.LittleEndian, &i) != nil {
					break
				}
				switch flag {
				case MUX_NEW_CONN: //new conn
					conn := NewConn(i, s, s.sendMsgCh, s.sendStatusCh)
					s.connMap.Set(i, conn) //it has been set before send ok
					s.newConnCh <- conn
					raw.Reset()
					binary.Write(raw, binary.LittleEndian, MUX_NEW_CONN_OK)
					binary.Write(raw, binary.LittleEndian, i)
					s.conn.Write(raw.Bytes())
					continue
				case MUX_PING_FLAG: //ping
					continue
				case MUX_NEW_MSG:
					if n, err = ReadLenBytes(buf, s.conn); err != nil {
						break
					}
				}
				if conn, ok := s.connMap.Get(i); ok && !conn.isClose {
					switch flag {
					case MUX_NEW_MSG: //new msg from remote conn
						copy(conn.readBuffer, buf[:n])
						conn.endRead = n
						if conn.readWait {
							conn.readCh <- struct{}{}
						}
					case MUX_MSG_SEND_OK: //the remote has read
						conn.getStatusCh <- struct{}{}
					case MUX_NEW_CONN_OK: //conn ok
						conn.connStatusOkCh <- struct{}{}
					case MUX_NEW_CONN_Fail:
						conn.connStatusFailCh <- struct{}{}
					case MUX_CONN_CLOSE: //close the connection
						conn.Close()
					}
				}
			} else {
				break
			}
		}
		s.Close()
	}()
	select {
	case <-s.closeChan:
	}
}

func (s *Mux) Close() error {
	if s.isClose {
		return errors.New("the mux has closed")
	}
	s.isClose = true
	s.connMap.Close()
	s.closeChan <- struct{}{}
	s.closeChan <- struct{}{}
	s.closeChan <- struct{}{}
	close(s.closeChan)
	close(s.sendMsgCh)
	close(s.sendStatusCh)
	return s.conn.Close()
}

//get new connId as unique flag
func (s *Mux) getId() int32 {
	return atomic.AddInt32(&s.id, 1)
}
