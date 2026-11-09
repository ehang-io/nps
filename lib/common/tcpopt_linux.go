//go:build linux
// +build linux

package common

import (
	"net"
	"syscall"
	"time"
)

// tcpUserTimeoutOpt 是 Linux 的 TCP_USER_TIMEOUT 选项号（include/netinet/tcp.h）。
// 直接用数值而不是引入 golang.org/x/sys，避免为一个常量把依赖链整体抬版本。
const tcpUserTimeoutOpt = 18

// SetUserTimeout 设置 TCP_USER_TIMEOUT：已发出的数据在被确认之前最多等多久，
// 超过就判定连接已死并返回 ETIMEDOUT。
//
// 这是 RDP（3389）最典型的「卡死」来源：客户端拔网线/切 Wi-Fi 之后连接没有正常关闭，
// 服务端仍在往一个不可达的对端重传，默认要等十几分钟才放弃，期间连接和协程一直占着。
// 2 分钟的用户超时足以覆盖正常的网络抖动，又能让失效会话尽快释放。
func SetUserTimeout(c net.Conn, d time.Duration) bool {
	if c == nil || d <= 0 {
		return false
	}
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return false
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		return false
	}
	ms := int(d / time.Millisecond)
	if ms <= 0 {
		return false
	}
	var serr error
	if err = raw.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpUserTimeoutOpt, ms)
	}); err != nil {
		return false
	}
	return serr == nil
}
