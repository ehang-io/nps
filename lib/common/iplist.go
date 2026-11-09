package common

import (
	"net"
	"strings"
)

// IPList 是一组 IP / CIDR / 前缀通配（如 192.168.1.*）的匹配集合。
// 用于给穿透端口做来源白名单、黑名单，避免公网扫描器直接摸到内网服务。
type IPList struct {
	all    bool
	rules  []ipRule
	rawStr string
}

type ipRule struct {
	ipNet  *net.IPNet //网段
	ip     net.IP     //单个地址
	prefix string     //前缀通配，例如 "192.168."
}

// NewIPList 解析逗号/分号/空白分隔的地址列表，空字符串表示不限制（Empty 为 true）
func NewIPList(raw string) *IPList {
	l := &IPList{rawStr: strings.TrimSpace(raw)}
	if l.rawStr == "" {
		return l
	}
	for _, token := range splitIPList(l.rawStr) {
		switch {
		case token == "*" || token == "all" || token == "any":
			l.all = true
		case strings.Contains(token, "/"):
			if _, n, err := net.ParseCIDR(token); err == nil {
				l.rules = append(l.rules, ipRule{ipNet: n})
			}
		case strings.Contains(token, "*"):
			idx := strings.Index(token, "*")
			l.rules = append(l.rules, ipRule{prefix: token[:idx]})
		default:
			if ip := net.ParseIP(token); ip != nil {
				l.rules = append(l.rules, ipRule{ip: ip})
			}
		}
	}
	return l
}

func splitIPList(s string) (out []string) {
	f := func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\r', '\n':
			return true
		}
		return false
	}
	for _, v := range strings.FieldsFunc(s, f) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Empty 表示列表为空，也就是不做任何限制
func (l *IPList) Empty() bool {
	return l == nil || (!l.all && len(l.rules) == 0)
}

// String 返回原始配置
func (l *IPList) String() string {
	if l == nil {
		return ""
	}
	return l.rawStr
}

// Contains 判断 ip 是否命中列表
func (l *IPList) Contains(ip net.IP) bool {
	if l == nil || ip == nil {
		return false
	}
	if l.all {
		return true
	}
	ip4 := ip.To4()
	for _, r := range l.rules {
		if r.ipNet != nil {
			if r.ipNet.Contains(ip) {
				return true
			}
			continue
		}
		if r.ip != nil {
			if r.ip.Equal(ip) || (ip4 != nil && r.ip.Equal(ip4)) {
				return true
			}
			continue
		}
		if r.prefix != "" {
			s := ip.String()
			if ip4 != nil {
				s = ip4.String()
			}
			if strings.HasPrefix(s, r.prefix) {
				return true
			}
		}
	}
	return false
}

// ContainsAddr 判断 net.Addr 是否命中列表，无法解析时按不命中处理
func (l *IPList) ContainsAddr(addr net.Addr) bool {
	return l.Contains(IPFromAddr(addr))
}

// IPFromAddr 从 net.Addr 里取出 IP，支持 TCPAddr/UDPAddr/字符串形式
func IPFromAddr(addr net.Addr) net.IP {
	if addr == nil {
		return nil
	}
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	}
	host := addr.String()
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return net.ParseIP(host)
}
