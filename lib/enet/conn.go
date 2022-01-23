package enet

import (
	"ehang.io/nps/lib/pool"
	"errors"
	"net"
	"sync"
	"syscall"
)

type Conn interface {
	net.Conn
	Reset(int) error
	Clear()
	Readable() bool
	AllBytes() ([]byte, error)
	SyscallConn() (syscall.RawConn, error)
}

var _ Conn = (*ReaderConn)(nil)

var bp = pool.NewBufferPool(MaxReadSize)

const MaxReadSize = 32 * 1024

// ReaderConn is an implement of reusable data connection
type ReaderConn struct {
	buf      []byte
	nowIndex int
	hasRead  int
	hasClear bool
	net.Conn
	sync.RWMutex
}

// NewReaderConn returns a new ReaderConn
func NewReaderConn(conn net.Conn) *ReaderConn {
	return &ReaderConn{Conn: conn, buf: bp.Get()}
}

// SyscallConn returns a raw network connection
func (rc *ReaderConn) SyscallConn() (syscall.RawConn, error) {
	return rc.Conn.(syscall.Conn).SyscallConn()
}

// Read is an implement of Net.Conn Read function
func (rc *ReaderConn) Read(b []byte) (n int, err error) {
	rc.Lock()
	defer rc.Unlock()
	if rc.hasClear || (rc.nowIndex == rc.hasRead && rc.hasRead == MaxReadSize) {
		if !rc.hasClear {
			rc.Clear()
		}
		return rc.Conn.Read(b)
	}
	if rc.hasRead > rc.nowIndex {
		n = copy(b, rc.buf[rc.nowIndex:rc.hasRead])
		rc.nowIndex += n
		return
	}
	if rc.hasRead == MaxReadSize {
		n = copy(b, rc.buf[rc.nowIndex:rc.hasRead])
		rc.nowIndex += n
		return
	}
	err = rc.readOnce()
	if err != nil {
		return
	}
	n = copy(b, rc.buf[rc.nowIndex:rc.hasRead])
	rc.nowIndex += n
	return
}

// readOnce
func (rc *ReaderConn) readOnce() error {
	// int(math.Min(float64(MaxReadSize-rc.hasRead), float64(len(b)-(rc.hasRead-rc.nowIndex))))
	// read as much as possible to judge whether there is still readable
	n, err := rc.Conn.Read(rc.buf[rc.nowIndex : rc.hasRead+MaxReadSize-rc.hasRead])
	rc.hasRead += n
	return err
}

// Readable return whether there is data in the buffer
func (rc *ReaderConn) Readable() bool {
	return (rc.hasRead - rc.nowIndex) > 0
}

// AllBytes return all data in the buffer
func (rc *ReaderConn) AllBytes() ([]byte, error) {
	rc.Lock()
	defer rc.Unlock()
	if rc.hasRead == 0 {
		if err := rc.readOnce(); err != nil {
			return nil, err
		}
	}
	if !rc.Readable() {
		return nil, errors.New("can not read '")
	}
	b := rc.buf[rc.nowIndex:rc.hasRead]
	rc.nowIndex = rc.hasRead
	return b, nil
}

// Reset will reset data index
func (rc *ReaderConn) Reset(n int) error {
	if !rc.hasClear {
		rc.nowIndex = n
		return nil
	}
	return errors.New("the enet can not reset anymore")
}

// Clear will put buf to pool and can not reuse anymore
func (rc *ReaderConn) Clear() {
	rc.hasClear = true
	bp.Put(rc.buf)
}
