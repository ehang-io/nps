package limiter

import (
	"ehang.io/nps/lib/enet"
	"github.com/pkg/errors"
	"net"
	"sync"
)

// ipNumMap is used to store the connection num of a ip address
type ipNumMap struct {
	m map[string]int32
	sync.Mutex
}

// AddOrSet is used to add connection num of a ip address
func (i *ipNumMap) AddOrSet(key string) {
	i.Lock()
	if v, ok := i.m[key]; ok {
		i.m[key] = v + 1
	} else {
		i.m[key] = 1
	}
	i.Unlock()
}

// SubOrDel is used to decrease connection of a ip address
func (i *ipNumMap) SubOrDel(key string) {
	i.Lock()
	if v, ok := i.m[key]; ok {
		i.m[key] = v - 1
		if i.m[key] == 0 {
			delete(i.m, key)
		}
	}
	i.Unlock()
}

// Get return the connection num of a ip
func (i *ipNumMap) Get(key string) int32 {
	return i.m[key]
}

// IpConnNumLimiter is used to limit the connection num of a service at the same time of same ip
type IpConnNumLimiter struct {
	m      *ipNumMap
	MaxNum int32 `json:"max_num" required:"true" placeholder:"10" zh_name:"单ip最大连接数"`
	sync.Mutex
}

func (cl *IpConnNumLimiter) GetName() string {
	return "ip_conn_num"
}

func (cl *IpConnNumLimiter) GetZhName() string {
	return "单ip连接数限制"
}

// Init the ipNumMap
func (cl *IpConnNumLimiter) Init() error {
	cl.m = &ipNumMap{m: make(map[string]int32)}
	return nil
}

// DoLimit reports whether the connection num of the ip exceed the maximum number
// If true, return error
func (cl *IpConnNumLimiter) DoLimit(c enet.Conn) (enet.Conn, error) {
	ip, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return c, errors.Wrap(err, "split ip addr")
	}
	if cl.m.Get(ip) >= cl.MaxNum {
		return c, errors.Errorf("the ip(%s) exceed the maximum number(%d)", ip, cl.MaxNum)
	}
	return NewNumConn(c, ip, cl.m), nil
}

// numConn is an implement of enet.Conn
type numConn struct {
	key string
	m   *ipNumMap
	enet.Conn
}

// NewNumConn return a numConn
func NewNumConn(c enet.Conn, key string, m *ipNumMap) *numConn {
	m.AddOrSet(key)
	return &numConn{
		m:    m,
		key:  key,
		Conn: c,
	}
}

// Close is used to decrease the connection num of a ip when connection closing
func (n *numConn) Close() error {
	n.m.SubOrDel(n.key)
	return n.Conn.Close()
}
