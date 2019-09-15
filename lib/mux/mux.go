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
	//logs.Warn("send mux new conn ", conn.connId)
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

func (s *Mux) sendInfo(flag uint8, id int32, data interface{}) {
	var err error
	if flag == common.MUX_NEW_MSG {
		if len(data.([]byte)) == 0 {
			//logs.Warn("send info content is nil")
		}
	}
	buf := common.BuffPool.Get()
	//defer pool.BuffPool.Put(buf)
	pack := common.MuxPack.Get()
	defer common.MuxPack.Put(pack)
	err = pack.NewPac(flag, id, data)
	if err != nil {
		//logs.Warn("new pack err", err)
		common.BuffPool.Put(buf)
		return
	}
	err = pack.Pack(buf)
	if err != nil {
		//logs.Warn("pack err", err)
		common.BuffPool.Put(buf)
		return
	}
	if pack.Flag == common.MUX_NEW_CONN {
		//logs.Warn("sendinfo mux new conn, insert to write queue", pack.Id)
	}
	s.writeQueue <- buf
	//_, err = buf.WriteTo(s.conn)
	//if err != nil {
	//	s.Close()
	//	logs.Warn("write err, close mux", err)
	//}
	//common.BuffPool.Put(buf)
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
				//logs.Warn("close from write session fail ", err, n, l)
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
		pack := common.MuxPack.Get()
		for {
			pack = common.MuxPack.Get()
			if pack.UnPack(s.conn) != nil {
				break
			}
			if pack.Flag != 0 && pack.Flag != 7 {
				if pack.Length > 10 {
					//logs.Warn(pack.Flag, pack.Id, pack.Length, string(pack.Content[:10]))
				}
			}
			if pack.Flag == common.MUX_NEW_CONN {
				//logs.Warn("unpack mux new connection", pack.Id)
			}
			s.pingOk = 0
			switch pack.Flag {
			case common.MUX_NEW_CONN: //new connection
				//logs.Warn("rec mux new connection", pack.Id)
				connection := NewConn(pack.Id, s)
				s.connMap.Set(pack.Id, connection) //it has been set before send ok
				go func(connection *conn) {
					connection.sendWindow.SetAllowSize(512) // set the initial receive window
				}(connection)
				s.newConnCh <- connection
				s.sendInfo(common.MUX_NEW_CONN_OK, connection.connId, nil)
				//logs.Warn("send mux new connection ok", connection.connId)
				continue
			case common.MUX_PING_FLAG: //ping
				//logs.Warn("send mux ping return")
				go s.sendInfo(common.MUX_PING_RETURN, common.MUX_PING, nil)
				continue
			case common.MUX_PING_RETURN:
				continue
			}
			if connection, ok := s.connMap.Get(pack.Id); ok && !connection.isClose {
				switch pack.Flag {
				case common.MUX_NEW_MSG: //new msg from remote connection
					//insert wait queue
					if connection.isClose {
						logs.Warn("rec mux new msg closed", pack.Id, string(pack.Content[0:15]))
						continue
					}
					connection.receiveWindow.WriteWg.Add(1)
					//logs.Warn("rec mux new msg ", connection.connId, string(pack.Content[0:15]))
					go func(connection *conn, content []byte) { // do not block read session
						_, err := connection.receiveWindow.Write(content)
						if err != nil {
							logs.Warn("mux new msg err close", err)
							connection.Close()
						}
						size := connection.receiveWindow.Size()
						if size == 0 {
							connection.receiveWindow.WindowFull = true
						}
						s.sendInfo(common.MUX_MSG_SEND_OK, connection.connId, size)
						//logs.Warn("send mux new msg ok", connection.connId, size)
						connection.receiveWindow.WriteWg.Done()
					}(connection, pack.Content)
					continue
				case common.MUX_NEW_CONN_OK: //connection ok
					//logs.Warn("rec mux new connection ok ", pack.Id)
					connection.connStatusOkCh <- struct{}{}
					go connection.sendWindow.SetAllowSize(512)
					// set the initial receive window both side
					continue
				case common.MUX_NEW_CONN_Fail:
					//logs.Warn("rec mux new connection fail", pack.Id)
					connection.connStatusFailCh <- struct{}{}
					continue
				case common.MUX_MSG_SEND_OK:
					if connection.isClose {
						//logs.Warn("rec mux msg send ok id window closed!", pack.Id, pack.Window)
						continue
					}
					//logs.Warn("rec mux msg send ok id window", pack.Id, pack.Window)
					go connection.sendWindow.SetAllowSize(pack.Window)
					continue
				case common.MUX_CONN_CLOSE: //close the connection
					//logs.Warn("rec mux connection close", pack.Id)
					s.connMap.Delete(pack.Id)
					connection.closeFlag = true
					go func(connection *conn) {
						//logs.Warn("receive mux connection close, wg waiting", connection.connId)
						connection.receiveWindow.WriteWg.Wait()
						//logs.Warn("receive mux connection close, wg waited", connection.connId)
						connection.receiveWindow.WriteEndOp <- struct{}{} // close signal to receive window
						//logs.Warn("receive mux connection close, finish", connection.connId)
					}(connection)
					continue
				}
			} else if pack.Flag == common.MUX_CONN_CLOSE {
				//logs.Warn("rec mux connection close no id ", pack.Id)
				continue
			}
			common.MuxPack.Put(pack)
		}
		common.MuxPack.Put(pack)
		//logs.Warn("read session put pack ", pack.Id)
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
