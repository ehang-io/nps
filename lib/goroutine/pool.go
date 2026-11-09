package goroutine

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/panjf2000/ants/v2"
)

// closeLingerAfterEOF 一半连接 EOF、另一半还在传数据时允许的空闲时间。
// 21/22/80 这类协议在请求发完后客户端会先关写方向（half-close），
// 服务端的响应往往紧接着就到：只要还有流量就继续等，空闲超过这个时间才收工，
// 既不会截断响应，也不会让连接在响应结束后继续占用 fd。
const closeLingerAfterEOF = 5 * time.Second

type connGroup struct {
	src  io.ReadWriteCloser
	dst  io.ReadWriteCloser
	wg   *sync.WaitGroup
	n    *int64
	last *int64 //最近一次成功写入的 unix nano，用于空闲判定
	eof  *int32 //任一方向已经正常结束写
}

func newConnGroup(dst, src io.ReadWriteCloser, wg *sync.WaitGroup, n, last *int64, eof *int32) connGroup {
	return connGroup{
		src:  src,
		dst:  dst,
		wg:   wg,
		n:    n,
		last: last,
		eof:  eof,
	}
}

func copyConnGroup(group interface{}) {
	cg, ok := group.(connGroup)
	if !ok {
		return
	}
	onWrite := func(int) {
		if cg.last != nil {
			atomic.StoreInt64(cg.last, time.Now().UnixNano())
		}
	}
	var err error
	*cg.n, err = common.CopyBufferTrack(cg.dst, cg.src, onWrite)
	if err != nil {
		if err == io.EOF {
			//一端正常结束写（FTP 传完、SSH 退出、HTTP 请求发完）：
			//能半关闭就只关写方向，交给反方向继续把响应发完；
			//mux 这类不支持半关闭的连接先不关，由 keepAliveTail 按空闲收尾
			if cg.eof != nil {
				atomic.StoreInt32(cg.eof, 1)
			}
			common.CloseWrite(cg.dst)
		} else {
			cg.src.Close()
			cg.dst.Close()
		}
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
	var in, out, last int64
	atomic.StoreInt64(&last, time.Now().UnixNano())
	// mux 连接不支持半关闭，eof 传 nil：一半 EOF 后由 keepAliveTail 按空闲收尾
	_ = connCopyPool.Invoke(newConnGroup(conns.conn1, conns.conn2, wg, &in, &last, nil))
	// outside to mux : incoming
	_ = connCopyPool.Invoke(newConnGroup(conns.conn2, conns.conn1, wg, &out, &last, nil))
	// mux to outside : outgoing
	done := make(chan struct{})
	go keepAliveTail(conns.conn1, conns.conn2, &last, done)
	wg.Wait()
	close(done)
	//两个方向都结束了必须关闭连接：原实现只统计流量没有关闭，
	//正常结束的转发会一直停在 CLOSE_WAIT，长时间运行会耗尽 fd 与内存
	closeQuietly(conns.conn1)
	closeQuietly(conns.conn2)
	if conns.flow != nil {
		conns.flow.Add(in, out)
	}
	conns.wg.Done()
}

// keepAliveTail 处理「一端已经 EOF，另一端还在回传数据」的收尾。
// 只要还有数据流动就继续等，连续 closeLingerAfterEOF 没有流量就关闭双方，
// 避免连接在响应结束后长时间挂着。
func keepAliveTail(a, b io.Closer, last *int64, done <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastSeen := atomic.LoadInt64(last)
	idleSince := time.Now()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			cur := atomic.LoadInt64(last)
			if cur != lastSeen {
				lastSeen = cur
				idleSince = time.Now()
				continue
			}
			if time.Since(idleSince) < closeLingerAfterEOF {
				continue
			}
			//两个方向都没有新数据了，直接收工
			closeQuietly(a)
			closeQuietly(b)
			return
		}
	}
}

//closeQuietly 关闭连接并忽略重复关闭的错误
func closeQuietly(c io.Closer) {
	if c == nil {
		return
	}
	_ = c.Close()
}

var connCopyPool, _ = ants.NewPoolWithFunc(200000, copyConnGroup, ants.WithNonblocking(false))
var CopyConnsPool, _ = ants.NewPoolWithFunc(100000, copyConns, ants.WithNonblocking(false))
