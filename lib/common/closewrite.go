package common

import (
	"crypto/tls"
	"io"
	"net"
	"time"
)

// CloseWriter 是支持半关闭写方向的连接。
// *net.TCPConn 与 *tls.Conn 都实现了它。
type CloseWriter interface {
	CloseWrite() error
}

// CloseWrite 尝试只关闭写方向，保留读方向。
// 21(FTP)/22(SSH)/80(HTTP) 这些协议在一方结束后还要继续读对端的收尾数据：
// 直接 Close 会把还没发完的响应截断，表现为“命令执行成功但客户端报连接被重置”。
func CloseWrite(c interface{}) bool {
	switch v := c.(type) {
	case *tls.Conn:
		return v.CloseWrite() == nil
	case *net.TCPConn:
		return v.CloseWrite() == nil
	case CloseWriter:
		return v.CloseWrite() == nil
	}
	return false
}

// CloseAfter 在 d 之后强制关闭给定的连接。
// 用于「一端已经 EOF、但对端不支持半关闭」的兜底：这段时间里让另一个方向把
// 剩余数据（例如 HTTP 响应）发完，超时后连进程一起收拾，避免连接一直挂着。
func CloseAfter(d time.Duration, conns ...io.Closer) *time.Timer {
	return time.AfterFunc(d, func() {
		for _, c := range conns {
			if c != nil {
				_ = c.Close()
			}
		}
	})
}
