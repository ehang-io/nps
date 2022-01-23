package limiter

import (
	"ehang.io/nps/lib/enet"
	"errors"
	"sync/atomic"
)

// FlowStore is an interface to store or get the flow now
type FlowStore interface {
	GetOutIn() (uint32, uint32)
	AddOut(out uint32) uint32
	AddIn(in uint32) uint32
}

// memStore is an implement of FlowStore
type memStore struct {
	nowOut uint32
	nowIn  uint32
}

// GetOutIn return out and in num 0
func (m *memStore) GetOutIn() (uint32, uint32) {
	return m.nowOut, m.nowIn
}

// AddOut is used to add out now
func (m *memStore) AddOut(out uint32) uint32 {
	return atomic.AddUint32(&m.nowOut, out)
}

// AddIn is used to add in now
func (m *memStore) AddIn(in uint32) uint32 {
	return atomic.AddUint32(&m.nowIn, in)
}

// FlowLimiter is used to limit the flow of a service
type FlowLimiter struct {
	Store    FlowStore
	OutLimit uint32 `json:"out_limit" required:"true" placeholder:"1024(kb)" zh_name:"出口最大流量"` //unit: kb, 0 means not limit
	InLimit  uint32 `json:"in_limit" required:"true" placeholder:"1024(kb)" zh_name:"入口最大流量"`  //unit: kb, 0 means not limit
}

func (f *FlowLimiter) GetName() string {
	return "flow"
}

func (f *FlowLimiter) GetZhName() string {
	return "流量限制"
}

// DoLimit return a flow limited enet.Conn
func (f *FlowLimiter) DoLimit(c enet.Conn) (enet.Conn, error) {
	return &flowConn{fl: f, Conn: c}, nil
}

// Init is used to set out or in num now
func (f *FlowLimiter) Init() error {
	if f.Store == nil {
		f.Store = &memStore{}
	}
	return nil
}

// flowConn is an implement of
type flowConn struct {
	enet.Conn
	fl *FlowLimiter
}

// Read add the in flow num of the service
func (fs *flowConn) Read(b []byte) (n int, err error) {
	n, err = fs.Conn.Read(b)
	if fs.fl.InLimit > 0 && fs.fl.Store.AddIn(uint32(n)) > fs.fl.InLimit {
		err = errors.New("exceed the in flow limit")
	}
	return
}

// Write add the out flow num of the service
func (fs *flowConn) Write(b []byte) (n int, err error) {
	n, err = fs.Conn.Write(b)
	if fs.fl.OutLimit > 0 && fs.fl.Store.AddOut(uint32(n)) > fs.fl.OutLimit {
		err = errors.New("exceed the out flow limit")
	}
	return
}
