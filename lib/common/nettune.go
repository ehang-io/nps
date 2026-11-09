package common

import (
	"crypto/tls"
	"net"
	"time"
)

// tuneKeepAlivePeriod 空闲连接的心跳周期。
// 3389/22/21 这类会话经常长时间没有数据（RDP 断开重连、SSH 挂起、FTP 控制通道空闲），
// 中间的 NAT / 云安全组默认 5~10 分钟就回收空闲条目，之后表现是“突然卡住”或必须重连。
// 30 秒一次心跳可以让映射一直活着，同时对带宽几乎无影响。
const tuneKeepAlivePeriod = 30 * time.Second

// TuneConn 为一条已建立的 TCP 连接套上适合内网穿透的参数：
//   - NoDelay：3389 这类交互式协议都是小包，关闭 Nagle 才能让按键/鼠标及时下发
//   - KeepAlive：避免空闲会话被 NAT 或防火墙静默丢弃
//
// 非 TCP 连接（kcp / mux 虚拟连接）原样返回。
func TuneConn(c net.Conn) net.Conn {
	return TuneConnForPort(c, 0)
}

// TuneConnForPort 在 TuneConn 的基础上按端口画像补充参数。
// 目前主要是 Linux 的 TCP_USER_TIMEOUT：RDP/SSH 的对端消失后不必等十几分钟才失败。
// port 传 0 或未收录端口时使用默认的失效连接回收时间。
func TuneConnForPort(c net.Conn, port int) net.Conn {
	if c == nil {
		return c
	}
	profile := ProfileForPort(port)
	userTimeout := profile.UserTimeout
	if userTimeout <= 0 {
		userTimeout = defaultUserTimeout
	}
	switch v := c.(type) {
	case *net.TCPConn:
		_ = v.SetNoDelay(true)
		_ = v.SetKeepAlive(true)
		_ = v.SetKeepAlivePeriod(tuneKeepAlivePeriod)
		SetUserTimeout(v, userTimeout)
	case *tls.Conn:
		//TLS 连在 TCP 之上，参数要设到下面的 TCP 上
		TuneConnForPort(v.NetConn(), port)
	}
	return c
}

// DialTuned 建立目标连接，使用带 KeepAlive 的拨号器。
// 与 net.DialTimeout 相比，握手失败会更快返回，并且连接建立后立即带上 KeepAlive，
// 目标端（如内网 3389/22）不会因为长时间静默被中间设备断开。
func DialTuned(network, address string, timeout time.Duration) (net.Conn, error) {
	return DialTunedForPort(network, address, timeout, 0)
}

// DialTunedForPort 与 DialTuned 相同，额外按目标端口画像设置连接参数。
// 客户端侧拨入内网 3389/22 时用得上：目标端口的失效连接回收时间与通用值不同。
func DialTunedForPort(network, address string, timeout time.Duration, port int) (net.Conn, error) {
	d := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: tuneKeepAlivePeriod,
	}
	c, err := d.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return TuneConnForPort(c, port), nil
}
