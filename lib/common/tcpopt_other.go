//go:build !linux
// +build !linux

package common

import (
	"net"
	"time"
)

// SetUserTimeout 在非 Linux 平台是空实现：
// Windows / macOS / BSD 没有与 TCP_USER_TIMEOUT 等价的、可移植的套接字选项，
// 那边靠 KeepAlive 与半关闭处理来回收失效连接，不在这里做平台特化的 syscall。
func SetUserTimeout(c net.Conn, d time.Duration) bool {
	return false
}
