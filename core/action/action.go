package action

import (
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/lib/pool"
	"errors"
	"go.uber.org/zap"
	"net"
	"sync"
)

var bp = pool.NewBufferPool(MaxReadSize)

const MaxReadSize = 32 * 1024

var (
	_ Action = (*AdminAction)(nil)
	_ Action = (*BridgeAction)(nil)
	_ Action = (*LocalAction)(nil)
	_ Action = (*NpcAction)(nil)
)

type Action interface {
	GetName() string
	GetZhName() string
	Init() error
	RunConnWithAddr(net.Conn, string) error
	RunConn(net.Conn) error
	GetServeConnWithAddr(string) (net.Conn, error)
	GetServerConn() (net.Conn, error)
	CanServe() bool
	RunPacketConn(conn net.PacketConn) error
}

type DefaultAction struct {
}

func (ba *DefaultAction) GetName() string {
	return "default"
}

func (ba *DefaultAction) GetZhName() string {
	return "默认"
}

func (ba *DefaultAction) Init() error {
	return nil
}

func (ba *DefaultAction) RunConn(clientConn net.Conn) error {
	return errors.New("not supported")
}

func (ba *DefaultAction) CanServe() bool {
	return false
}

func (ba *DefaultAction) RunPacketConn(conn net.PacketConn) error {
	return errors.New("not supported")
}

func (ba *DefaultAction) GetServerConn() (net.Conn, error) {
	return nil, errors.New("can not get component connection")
}

func (ba *DefaultAction) GetServeConnWithAddr(addr string) (net.Conn, error) {
	return nil, errors.New("can not get component connection")
}

func (ba *DefaultAction) startCopy(c1 net.Conn, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	err := pool.CopyConnGoroutinePool.Invoke(&pool.CopyConnGpParams{
		Reader: c2,
		Writer: c1,
		Wg:     &wg,
	})
	if err != nil {
		logger.Error("Invoke goroutine failed", zap.Error(err))
		return
	}
	buf := bp.Get()
	_, _ = common.CopyBuffer(c2, c1, buf)
	bp.Put(buf)
	if v, ok := c1.(*net.TCPConn); ok {
		_ = v.CloseRead()
	}
	if v, ok := c2.(*net.TCPConn); ok {
		_ = v.CloseWrite()
	}
	wg.Wait()
}

func (ba *DefaultAction) startCopyPacketConn(p1 net.PacketConn, p2 net.PacketConn) error {
	var wg sync.WaitGroup
	wg.Add(2)
	_ = pool.CopyPacketGoroutinePool.Invoke(&pool.CopyPacketGpParams{
		RPacket: p1,
		WPacket: p2,
		Wg:      &wg,
	})
	_ = pool.CopyPacketGoroutinePool.Invoke(&pool.CopyPacketGpParams{
		RPacket: p2,
		WPacket: p1,
		Wg:      &wg,
	})
	wg.Wait()
	return nil
}
