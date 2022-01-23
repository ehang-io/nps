package enet

import (
	"bytes"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/pool"
	"github.com/pkg/errors"
	"net"
	"sync/atomic"
	"time"
)

var (
	_ net.PacketConn = (*TcpPacketConn)(nil)
	_ PacketConn     = (*ReaderPacketConn)(nil)
)

type PacketConn interface {
	net.PacketConn
	SendPacket([]byte, net.Addr) error
	FirstPacket() ([]byte, net.Addr, error)
}

var udpBp = pool.NewBufferPool(1500)

// TcpPacketConn is an implement of net.PacketConn by net.Conn
type TcpPacketConn struct {
	udpBp []byte
	net.Conn
}

// NewTcpPacketConn return a *TcpPacketConn
func NewTcpPacketConn(conn net.Conn) *TcpPacketConn {
	return &TcpPacketConn{Conn: conn}
}

// ReadFrom is a implement of net.PacketConn ReadFrom
func (tp *TcpPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	b := udpBp.Get()
	defer udpBp.Put(b)
	n, err = common.ReadLenBytes(tp.Conn, b)
	if err != nil {
		return
	}
	rAddr, err := common.ReadAddr(bytes.NewReader(b[:n]))
	if err != nil {
		return
	}
	n = copy(p, b[len(rAddr):n])
	addr, err = net.ResolveUDPAddr("udp", rAddr.String())
	return
}

// WriteTo is a implement of net.PacketConn WriteTo
func (tp *TcpPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	var pAddr common.Addr
	pAddr, err = common.ParseAddr(addr.String())
	if err != nil {
		return
	}
	return common.WriteLenBytes(tp.Conn, append(pAddr, p...))
}

// ReaderPacketConn is an implementation of net.PacketConn
type ReaderPacketConn struct {
	ch              chan *packet
	closeCh         chan struct{}
	closed          int32
	nowNum          int32
	addr            net.Addr
	writePacketConn net.PacketConn
	readTimer       *time.Timer
	firstPacket     []byte
}

type packet struct {
	b    []byte
	addr net.Addr
}

// NewReaderPacketConn returns an initialized PacketConn
func NewReaderPacketConn(writePacketConn net.PacketConn, firstPacket []byte, addr net.Addr) *ReaderPacketConn {
	return &ReaderPacketConn{
		ch:              make(chan *packet, 10),
		closeCh:         make(chan struct{}),
		addr:            addr,
		writePacketConn: writePacketConn,
		readTimer:       time.NewTimer(time.Hour * 24 * 3650),
		firstPacket:     firstPacket,
	}
}

func (pc *ReaderPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var pt *packet
	select {
	case pt = <-pc.ch:
	case <-pc.readTimer.C:
	}
	if pt == nil {
		return 0, nil, errors.New("the PacketConn is already closed")
	}
	copy(p, pt.b)
	return len(pt.b), pt.addr, nil
}

func (pc *ReaderPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return pc.writePacketConn.WriteTo(p, addr)
}

// LocalAddr returns the listener's address
func (pc *ReaderPacketConn) LocalAddr() net.Addr {
	return pc.addr
}

func (pc *ReaderPacketConn) SetDeadline(t time.Time) error {
	pc.readTimer.Reset(t.Sub(time.Now()))
	return pc.writePacketConn.SetWriteDeadline(t)
}

func (pc *ReaderPacketConn) SetReadDeadline(t time.Time) error {
	pc.readTimer.Reset(t.Sub(time.Now()))
	return nil
}

func (pc *ReaderPacketConn) SetWriteDeadline(t time.Time) error {
	return pc.writePacketConn.SetWriteDeadline(t)
}

func (pc *ReaderPacketConn) FirstPacket() ([]byte, net.Addr, error) {
	if pc.firstPacket == nil || pc.addr == nil {
		return nil, nil, errors.New("not found first packet")
	}
	return pc.firstPacket, pc.addr, nil
}

// SendPacket is used to add connection to the listener
func (pc *ReaderPacketConn) SendPacket(b []byte, addr net.Addr) error {
	if atomic.LoadInt32(&pc.closed) == 1 {
		return errors.New("the listener is already closed")
	}
	atomic.AddInt32(&pc.nowNum, 1)
	select {
	case pc.ch <- &packet{b: b, addr: addr}:
		return nil
	case <-pc.closeCh:
	case <-pc.readTimer.C:
		_ = pc.Close()
	}
	if atomic.AddInt32(&pc.nowNum, -1) == 0 && atomic.LoadInt32(&pc.closed) == 1 {
		close(pc.ch)
	}
	return errors.New("the packetConn is already closed")
}

// Close is used to close the listener, it will discard all  existing connections
func (pc *ReaderPacketConn) Close() error {
	if atomic.CompareAndSwapInt32(&pc.closed, 0, 1) {
		close(pc.closeCh)
	}
	return nil
}
