package mux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
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
	MUX_PING_RETURN
)

type Mux struct {
	net.Listener
	conn      net.Conn
	connMap   *connMap
	newConnCh chan *conn
	id        int32
	closeChan chan struct{}
	IsClose   bool
	sync.Mutex
}

func NewMux(c net.Conn) *Mux {
	m := &Mux{
		conn:      c,
		connMap:   NewConnMap(),
		id:        0,
		closeChan: make(chan struct{}),
		newConnCh: make(chan *conn),
		IsClose:   false,
	}
	//read session by flag
	go m.readSession()
	//ping
	go m.ping()
	return m
}

func (s *Mux) NewConn() (*conn, error) {
	if s.IsClose {
		return nil, errors.New("the mux has closed")
	}
	conn := NewConn(s.getId(), s)
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
	//set a timer timeout 30 second
	timer := time.NewTimer(time.Second * 30)
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
		return nil, errors.New("accpet error,the conn has closed")
	}
	return <-s.newConnCh, nil
}

func (s *Mux) Addr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *Mux) sendInfo(flag int32, id int32, content []byte) error {
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, flag)
	binary.Write(raw, binary.LittleEndian, id)
	if content != nil && len(content) > 0 {
		binary.Write(raw, binary.LittleEndian, int32(len(content)))
		binary.Write(raw, binary.LittleEndian, content)
	}
	if _, err := s.conn.Write(raw.Bytes()); err != nil {
		s.Close()
		return err
	}
	return nil
}

func (s *Mux) ping() {
	go func() {
		ticker := time.NewTicker(time.Second * 1)
		for {
			select {
			case <-ticker.C:
			}
			//Avoid going beyond the scope
			if (math.MaxInt32 - s.id) < 10000 {
				s.id = 0
			}
			if err := s.sendInfo(MUX_PING_FLAG, MUX_PING, nil); err != nil {
				logs.Error("ping error,close the connection")
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
	var buf []byte
	go func() {
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
					conn := NewConn(i, s)
					s.connMap.Set(i, conn) //it has been set before send ok
					s.newConnCh <- conn
					s.sendInfo(MUX_NEW_CONN_OK, i, nil)
					continue
				case MUX_PING_FLAG: //ping
					s.sendInfo(MUX_PING_RETURN, MUX_PING, nil)
					continue
				case MUX_PING_RETURN:
					continue
				case MUX_NEW_MSG:
					buf = pool.GetBufPoolCopy()
					if n, err = ReadLenBytes(buf, s.conn); err != nil {
						break
					}
				}
				if conn, ok := s.connMap.Get(i); ok && !conn.isClose {
					switch flag {
					case MUX_NEW_MSG: //new msg from remote conn
						//insert wait queue
						conn.waitQueue.Push(NewBufNode(buf, n))
						//judge len if >xxx ,send stop
						if conn.readWait {
							conn.readWait = false
							conn.readCh <- struct{}{}
						}
					case MUX_MSG_SEND_OK: //the remote has read
						select {
						case conn.getStatusCh <- struct{}{}:
						default:
						}
						conn.hasWrite --
					case MUX_NEW_CONN_OK: //conn ok
						conn.connStatusOkCh <- struct{}{}
					case MUX_NEW_CONN_Fail:
						conn.connStatusFailCh <- struct{}{}
					case MUX_CONN_CLOSE: //close the connection
						go conn.Close()
						s.connMap.Delete(i)
					}
				} else if flag == MUX_NEW_MSG {
					pool.PutBufPoolCopy(buf)
				}
			} else {
				logs.Error("read or send error")
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
	if s.IsClose {
		return errors.New("the mux has closed")
	}
	s.IsClose = true
	s.connMap.Close()
	select {
	case s.closeChan <- struct{}{}:
	}
	select {
	case s.closeChan <- struct{}{}:
	}
	return s.conn.Close()
}

//get new connId as unique flag
func (s *Mux) getId() int32 {
	return atomic.AddInt32(&s.id, 1)
}
