package mux

import (
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"
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
	connType   string
	writeQueue *sliceEntry
	sync.Mutex
}

func NewMux(c net.Conn, connType string) *Mux {
	m := &Mux{
		conn:       c,
		connMap:    NewConnMap(),
		id:         0,
		closeChan:  make(chan struct{}),
		newConnCh:  make(chan *conn),
		IsClose:    false,
		connType:   connType,
		writeQueue: NewQueue(),
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
	//it must be set before send
	s.connMap.Set(conn.connId, conn)
	if err := s.sendInfo(common.MUX_NEW_CONN, conn.connId, nil); err != nil {
		return nil, err
	}
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
		return nil, errors.New("accpet error,the conn has closed")
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

func (s *Mux) sendInfo(flag uint8, id int32, content []byte) (err error) {
	buf := pool.BuffPool.Get()
	defer pool.BuffPool.Put(buf)
	pack := common.MuxPackager{}
	err = pack.NewPac(flag, id, content)
	if err != nil {
		s.Close()
		logs.Warn("new pack err", err)
		return
	}
	err = pack.Pack(buf)
	if err != nil {
		s.Close()
		logs.Warn("pack err", err)
		return
	}
	_, err = buf.WriteTo(s.conn)
	if err != nil {
		s.Close()
		logs.Warn("write err", err)
	}
	return
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
			if err := s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, nil); err != nil || (s.pingOk > 10 && s.connType == "kcp") {
				s.Close()
				break
			}
			s.pingOk++
		}
	}()
	select {
	case <-s.closeChan:
	}
}

func (s *Mux) readSession() {
	var pack common.MuxPackager
	go func() {
		for {
			if pack.UnPack(s.conn) != nil {
				break
			}
			s.pingOk = 0
			switch pack.Flag {
			case common.MUX_NEW_CONN: //new conn
				logs.Warn("mux new conn", pack.Id)
				conn := NewConn(pack.Id, s)
				s.connMap.Set(pack.Id, conn) //it has been set before send ok
				s.newConnCh <- conn
				s.sendInfo(common.MUX_NEW_CONN_OK, pack.Id, nil)
				continue
			case common.MUX_PING_FLAG: //ping
				s.sendInfo(common.MUX_PING_RETURN, common.MUX_PING, nil)
				continue
			case common.MUX_PING_RETURN:
				continue
			}
			if conn, ok := s.connMap.Get(pack.Id); ok && !conn.isClose {
				switch pack.Flag {
				case common.MUX_NEW_MSG: //new msg from remote conn
					//insert wait queue
					conn.waitQueue.Push(NewBufNode(pack.Content, int(pack.Length)))
					//judge len if >xxx ,send stop
					if conn.readWait {
						conn.readWait = false
						conn.readCh <- struct{}{}
					}
				case common.MUX_NEW_CONN_OK: //conn ok
					conn.connStatusOkCh <- struct{}{}
				case common.MUX_NEW_CONN_Fail:
					conn.connStatusFailCh <- struct{}{}
				case common.MUX_CONN_CLOSE: //close the connection
					conn.waitQueue.Push(NewBufNode(nil, 0))
					s.connMap.Delete(pack.Id)
				}
			} else if pack.Flag == common.MUX_NEW_MSG {
				pool.PutBufPoolCopy(pack.Content)
			}
		}
		s.Close()
	}()
	select {
	case <-s.closeChan:
	}
}

func (s *Mux) Close() error {
	logs.Warn("close mux")
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
