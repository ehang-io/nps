package goroutine

import (
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/panjf2000/ants/v2"
	"io"
	"net"
	"sync"
)

type connGroup struct {
	src io.ReadWriteCloser
	dst io.ReadWriteCloser
	wg  *sync.WaitGroup
	n   *int64
}

func newConnGroup(dst, src io.ReadWriteCloser, wg *sync.WaitGroup, n *int64) connGroup {
	return connGroup{
		src: src,
		dst: dst,
		wg:  wg,
		n:   n,
	}
}

func copyConnGroup(group interface{}) {
	cg, ok := group.(connGroup)
	if !ok {
		return
	}
	var err error
	*cg.n, err = common.CopyBuffer(cg.dst, cg.src)
	if err != nil {
		cg.src.Close()
		cg.dst.Close()
		//logs.Warn("close npc by copy from nps", err, c.connId)
	}
	cg.wg.Done()
}

type Conns struct {
	conn1 io.ReadWriteCloser // mux connection
	conn2 net.Conn           // outside connection
	flow  *file.Flow
	wg    *sync.WaitGroup
}

func NewConns(c1 io.ReadWriteCloser, c2 net.Conn, flow *file.Flow, wg *sync.WaitGroup) Conns {
	return Conns{
		conn1: c1,
		conn2: c2,
		flow:  flow,
		wg:    wg,
	}
}

func copyConns(group interface{}) {
	conns := group.(Conns)
	wg := new(sync.WaitGroup)
	wg.Add(2)
	var in, out int64
	_ = connCopyPool.Invoke(newConnGroup(conns.conn1, conns.conn2, wg, &in))
	// outside to mux : incoming
	_ = connCopyPool.Invoke(newConnGroup(conns.conn2, conns.conn1, wg, &out))
	// mux to outside : outgoing
	wg.Wait()
	if conns.flow != nil {
		conns.flow.Add(in, out)
	}
	conns.wg.Done()
}

var connCopyPool, _ = ants.NewPoolWithFunc(200000, copyConnGroup, ants.WithNonblocking(false))
var CopyConnsPool, _ = ants.NewPoolWithFunc(100000, copyConns, ants.WithNonblocking(false))
