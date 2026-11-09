package common

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// ErrNoFirstByte 表示连接在约定时间内没有发出任何应用层数据，按扫描器/空连接处理。
var ErrNoFirstByte = errors.New("no data within first byte timeout")

var firstBytePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 1024)
		return &b
	},
}

// firstByteConn 把已经读出来的首包回放给后续的读取方。
type firstByteConn struct {
	net.Conn
	once    sync.Once
	pending []byte
}

func (c *firstByteConn) Read(b []byte) (int, error) {
	if len(c.pending) > 0 {
		n := copy(b, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
	return c.Conn.Read(b)
}

// AwaitFirstByte 在 d 时间内等待连接出现第一个应用层字节，并把读到的内容回放出去。
//
// 目的：3389/22/80/443 这类协议都是「客户端先说话」。端口扫描器、指纹探测脚本
// 建立 TCP 连接后往往什么都不发（或者只发一个空包等 banner），这类连接会一直挂在
// 服务端直到超时，既占连接数也暴露端口真实状态。先等一次首包：
//   - 正常客户端毫秒级就发出首包，后续转发完全不受影响
//   - 超时仍无数据的连接直接关闭，扫描器看到的是「连上就被关」而不是一个活着的服务
//
// 已通过 TLS / Basic 认证的连接不需要再调用：认证握手本身已经消费了首包。
func AwaitFirstByte(c net.Conn, d time.Duration) (net.Conn, error) {
	if c == nil {
		return nil, errors.New("nil connection")
	}
	if d <= 0 {
		return c, nil
	}
	bufp := firstBytePool.Get().(*[]byte)
	buf := *bufp
	defer firstBytePool.Put(bufp)

	if err := c.SetReadDeadline(time.Now().Add(d)); err != nil {
		// 不支持设置 deadline 的连接（mux 虚拟连接等）不做首包判定
		return c, nil
	}
	n, err := c.Read(buf)
	// 首包判定结束，必须清掉 deadline，否则后续正常转发会在 d 之后被误杀
	_ = c.SetReadDeadline(time.Time{})
	if n > 0 {
		pending := make([]byte, n)
		copy(pending, buf[:n])
		return &firstByteConn{Conn: c, pending: pending}, nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		return nil, ErrNoFirstByte
	}
	// 读超时同样按「没有首包」处理，调用方只需判断 ErrNoFirstByte
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return nil, ErrNoFirstByte
	}
	return nil, err
}
