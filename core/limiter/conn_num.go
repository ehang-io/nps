package limiter

import (
	"ehang.io/nps/lib/enet"
	"errors"
	"sync/atomic"
)

// ConnNumLimiter is used to limit the connection num of a service
type ConnNumLimiter struct {
	baseLimiter
	nowNum     int32
	MaxConnNum int32 `json:"max_conn_num" required:"true" placeholder:"10" zh_name:"最大连接数"` //0 means not limit
}

func (cl *ConnNumLimiter) GetName() string {
	return "conn_num"
}

func (cl *ConnNumLimiter) GetZhName() string {
	return "总连接数限制"
}

// DoLimit return an error if the connection num exceed the maximum
func (cl *ConnNumLimiter) DoLimit(c enet.Conn) (enet.Conn, error) {
	if atomic.AddInt32(&cl.nowNum, 1) > cl.MaxConnNum && cl.MaxConnNum > 0 {
		atomic.AddInt32(&cl.nowNum, -1)
		return nil, errors.New("exceed maximum number of connections")
	}
	return &connNumConn{nowNum: &cl.nowNum}, nil
}

// connNumConn is an implementation of enet.Conn
type connNumConn struct {
	nowNum *int32
	enet.Conn
}

// Close decrease the connection num
func (cn *connNumConn) Close() error {
	atomic.AddInt32(cn.nowNum, -1)
	return cn.Conn.Close()
}
