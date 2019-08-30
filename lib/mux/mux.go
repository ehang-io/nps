package mux

import (
	"bytes"
	"errors"
	"github.com/cnlh/nps/lib/common"
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
	writeQueue chan *bytes.Buffer
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
		writeQueue: make(chan *bytes.Buffer, 20),
	}
	//read session by flag
	go m.readSession()
	//ping
	go m.ping()
	go m.writeSession()
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
	logs.Warn("send mux new conn ", conn.connId)
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

func (s *Mux) sendInfo(flag uint8, id int32, content []byte) {
	var err error
	if flag == common.MUX_NEW_MSG {
		if len(content) == 0 {
			logs.Warn("send info content is nil")
		}
	}
	buf := common.BuffPool.Get()
	//defer pool.BuffPool.Put(buf)
	pack := common.MuxPack.Get()
	err = pack.NewPac(flag, id, content)
	if err != nil {
		s.Close()
		logs.Warn("new pack err", err)
		common.BuffPool.Put(buf)
		return
	}
	err = pack.Pack(buf)
	if err != nil {
		s.Close()
		logs.Warn("pack err", err)
		common.BuffPool.Put(buf)
		return
	}
	s.writeQueue <- buf
	common.MuxPack.Put(pack)
	//_, err = buf.WriteTo(s.conn)
	//if err != nil {
	//	s.Close()
	//	logs.Warn("write err, close mux", err)
	//}
	//if flag == common.MUX_CONN_CLOSE {
	//}
	//if flag == common.MUX_NEW_MSG {
	//}
	return
}

func (s *Mux) writeSession() {
	go func() {
		for {
			buf := <-s.writeQueue
			l := buf.Len()
			n, err := buf.WriteTo(s.conn)
			common.BuffPool.Put(buf)
			if err != nil || int(n) != l {
				logs.Warn("close from write to ", err, n, l)
				s.Close()
				break
			}
		}
	}()
	<-s.closeChan
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
			//logs.Warn("send mux ping")
			s.sendInfo(common.MUX_PING_FLAG, common.MUX_PING, nil)
			if s.pingOk > 10 && s.connType == "kcp" {
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
	go func() {
		for {
			pack := common.MuxPack.Get()
			if pack.UnPack(s.conn) != nil {
				break
			}
			if pack.Flag != 0 && pack.Flag != 7 {
				if pack.Length > 10 {
					//logs.Warn(pack.Flag, pack.Id, pack.Length, string(pack.Content[:10]))
				}
			}
			s.pingOk = 0
			switch pack.Flag {
			case common.MUX_NEW_CONN: //new conn
				logs.Warn("mux new conn", pack.Id)
				conn := NewConn(pack.Id, s)
				s.connMap.Set(pack.Id, conn) //it has been set before send ok
				s.newConnCh <- conn
				s.sendInfo(common.MUX_NEW_CONN_OK, pack.Id, nil)
				logs.Warn("send mux new conn ok", pack.Id)
				continue
			case common.MUX_PING_FLAG: //ping
				//logs.Warn("send mux ping return")
				go s.sendInfo(common.MUX_PING_RETURN, common.MUX_PING, nil)
				continue
			case common.MUX_PING_RETURN:
				continue
			}
			if conn, ok := s.connMap.Get(pack.Id); ok && !conn.isClose {
				switch pack.Flag {
				case common.MUX_NEW_MSG: //new msg from remote conn
					//insert wait queue
					logs.Warn("mux new msg ", pack.Id)
					conn.readQueue.Push(NewBufNode(pack.Content, int(pack.Length)))
					//judge len if >xxx ,send stop
					if conn.readWait {
						conn.readWait = false
						conn.readCh <- struct{}{}
					}
				case common.MUX_NEW_CONN_OK: //conn ok
					logs.Warn("mux new conn ok ", pack.Id)
					conn.connStatusOkCh <- struct{}{}
				case common.MUX_NEW_CONN_Fail:
					logs.Warn("mux new conn fail", pack.Id)
					conn.connStatusFailCh <- struct{}{}
				case common.MUX_CONN_CLOSE: //close the connection
					logs.Warn("mux conn close", pack.Id)
					s.connMap.Delete(pack.Id)
					conn.writeClose = true
					conn.readQueue.Push(NewBufNode(nil, 0))
					if conn.readWait {
						logs.Warn("close read wait", pack.Id)
						conn.readWait = false
						conn.readCh <- struct{}{}
					}
					logs.Warn("receive mux conn close, finish", conn.connId)
				}
			} else if pack.Flag == common.MUX_NEW_MSG {
				common.CopyBuff.Put(pack.Content)
			} else if pack.Flag == common.MUX_CONN_CLOSE {
				logs.Warn("mux conn close no id ", pack.Id)
			}
			common.MuxPack.Put(pack)
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
	s.closeChan <- struct{}{}
	close(s.writeQueue)
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
