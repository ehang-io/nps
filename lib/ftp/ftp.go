// Package ftp 解析并改写 FTP 控制通道里与数据通道相关的命令和响应。
//
// 内网穿透场景下，FTP 的数据通道有三种典型问题：
//   - 被动模式（PASV/EPSV）：FTP 服务在 227/229 里通告的是内网地址和一个随机端口，
//     公网客户端即使能连上控制通道，也连不上数据通道
//   - 主动模式（PORT/EPRT）：FTP 服务要反过来连客户端通告的地址，而这个地址在
//     客户端位于内网时不可达（这是 FTP 协议本身的限制，不是穿透的问题）
//   - 两种模式的数据端口都是临时端口，无法在防火墙/安全组上提前放行
//
// 服务端只要把这两类报文里的地址和端口改写成一个自己能控制的固定端口，
// 就能让数据通道走同一个隧道。本包只做纯文本解析，不碰网络，便于单测。
package ftp

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// MaxLine 是控制通道单行的最大长度。超过这个长度的流量按二进制处理
// （例如 AUTH TLS 之后的加密数据），调用方应停止改写并回退为直通。
const MaxLine = 8 * 1024

// Verb 返回命令/响应的动词（大写）。响应如 "227 Entering..." 返回 "227"。
//
// 多行响应的续行首位形如 "227-Entering Passive Mode (...)"，用 '-' 而不是空格
// 分隔，这里同样归一成三位响应码，否则 ProFTPD 一类服务的多行 227 会被漏掉，
// 数据通道地址就原样泄漏给公网客户端。
func Verb(line string) string {
	s := strings.Trim(strings.TrimRight(line, "\r\n"), " \t")
	if s == "" {
		return ""
	}
	if isReplyCode(s) {
		return s[:3]
	}
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return strings.ToUpper(s[:i])
	}
	return strings.ToUpper(s)
}

// isReplyCode 判断整行是否以三位响应码开头（"227"、"227 ..."、"227-..."）
func isReplyCode(s string) bool {
	if len(s) < 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if len(s) == 3 {
		return true
	}
	switch s[3] {
	case ' ', '\t', '-':
		return true
	}
	return false
}

// args 返回动词之后的参数部分（已去掉行尾）
func args(line string) string {
	s := strings.TrimRight(line, "\r\n")
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return ""
}

// eol 返回行尾的换行符（\r\n、\n 或空），改写时原样保留
func eol(line string) string {
	s := strings.TrimRight(line, "\r\n")
	return line[len(s):]
}

// 取最后一对括号里的内容
func parenContent(s string) (string, bool) {
	i := strings.LastIndex(s, "(")
	j := strings.LastIndex(s, ")")
	if i < 0 || j <= i {
		return "", false
	}
	return s[i+1 : j], true
}

// ipv4ToNumbers 把 IPv4 转成 PASV 需要的 "a,b,c,d" 形式
func ipv4ToNumbers(host string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return "", false
	}
	v4 := ip.To4()
	if v4 == nil {
		return "", false
	}
	return fmt.Sprintf("%d,%d,%d,%d", v4[0], v4[1], v4[2], v4[3]), true
}

// 解析 "a,b,c,d,p1,p2" 六个 0-255 的数字
func parseSix(s string) (host string, port int, ok bool) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 6 {
		return "", 0, false
	}
	nums := make([]int, 6)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 || n > 255 {
			return "", 0, false
		}
		nums[i] = n
	}
	port = nums[4]<<8 | nums[5]
	if port <= 0 {
		return "", 0, false
	}
	return fmt.Sprintf("%d.%d.%d.%d", nums[0], nums[1], nums[2], nums[3]), port, true
}

// pasvRegexp 匹配不带括号的 "a,b,c,d,p1,p2"。
// 少数 FTP 服务（多为嵌入式实现）在 227 响应里直接给出六段数字而不加括号。
var pasvRegexp = regexp.MustCompile(`(?:\d{1,3},\s*){5}\d{1,3}`)

// pasvSpan 定位 227 响应里数据通道地址所占的区间。标准带括号形式优先，
// 其次是不带括号的数字序列；改写时只替换这段区间，前缀文本与行尾保持不变。
func pasvSpan(line string) (start, end int, host string, port int, ok bool) {
	body := strings.TrimRight(line, "\r\n")
	if inside, found := parenContent(body); found {
		if h, p, ok2 := parseSix(inside); ok2 {
			i := strings.LastIndex(body, "(")
			return i + 1, i + 1 + len(inside), h, p, true
		}
	}
	locs := pasvRegexp.FindAllStringIndex(body, -1)
	// 从右往左取，避免前面出现别的数字（例如 "227 Passive mode a=1 1,2,3,4,5,6"）
	for i := len(locs) - 1; i >= 0; i-- {
		if h, p, ok2 := parseSix(body[locs[i][0]:locs[i][1]]); ok2 {
			return locs[i][0], locs[i][1], h, p, true
		}
	}
	return 0, 0, "", 0, false
}

// ParsePasv 解析 227 响应里通告的数据通道地址，例如：
//
//	227 Entering Passive Mode (192,168,1,5,19,136).
func ParsePasv(line string) (host string, port int, ok bool) {
	if Verb(line) != "227" {
		return "", 0, false
	}
	_, _, host, port, ok = pasvSpan(line)
	return host, port, ok
}

// SetPasv 把 227 响应里的地址替换为 host:port，其余文本与行尾保持不动
func SetPasv(line, host string, port int) (string, bool) {
	if port <= 0 || port > 65535 {
		return line, false
	}
	nums, ok := ipv4ToNumbers(host)
	if !ok {
		return line, false
	}
	start, end, _, _, ok := pasvSpan(line)
	if !ok {
		return line, false
	}
	body := strings.TrimRight(line, "\r\n")
	if start < 0 || end > len(body) || start >= end {
		return line, false
	}
	inside := fmt.Sprintf("%s,%d,%d", nums, port>>8, port&0xff)
	return body[:start] + inside + body[end:] + eol(line), true
}

// ParseEpsv 解析 229 响应里通告的数据端口，例如：
//
//	229 Entering Extended Passive Mode (|||6446|)
func ParseEpsv(line string) (port int, ok bool) {
	if Verb(line) != "229" {
		return 0, false
	}
	body := strings.TrimRight(line, "\r\n")
	i := strings.Index(body, "(|||")
	if i < 0 {
		return 0, false
	}
	rest := body[i+4:]
	j := strings.Index(rest, "|")
	if j <= 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:j]))
	if err != nil || n <= 0 || n > 65535 {
		return 0, false
	}
	return n, true
}

// SetEpsv 把 229 响应里的端口替换为 port
func SetEpsv(line string, port int) (string, bool) {
	if port <= 0 || port > 65535 {
		return line, false
	}
	body := strings.TrimRight(line, "\r\n")
	i := strings.Index(body, "(|||")
	if i < 0 {
		return line, false
	}
	rest := body[i+4:]
	j := strings.Index(rest, "|")
	if j <= 0 {
		return line, false
	}
	return body[:i] + fmt.Sprintf("(|||%d|", port) + rest[j+1:] + eol(line), true
}

// ParsePort 解析主动模式的 PORT 命令，例如：
//
//	PORT 192,168,1,10,19,136
func ParsePort(line string) (addr string, ok bool) {
	if Verb(line) != "PORT" {
		return "", false
	}
	host, port, ok := parseSix(args(line))
	if !ok {
		return "", false
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), true
}

// SetPort 把 PORT 命令里的地址替换为 host:port
func SetPort(line, host string, port int) (string, bool) {
	if port <= 0 || port > 65535 {
		return line, false
	}
	nums, ok := ipv4ToNumbers(host)
	if !ok {
		return line, false
	}
	body := strings.TrimRight(line, "\r\n")
	i := strings.IndexAny(body, " \t")
	if i < 0 {
		return line, false
	}
	return body[:i] + fmt.Sprintf(" %s,%d,%d", nums, port>>8, port&0xff) + eol(line), true
}

// ParseEprt 解析主动模式的 EPRT 命令（支持 IPv4/IPv6），例如：
//
//	EPRT |1|192.168.1.10|5000|
//	EPRT |2|2001:db8::1|5000|
func ParseEprt(line string) (addr string, ok bool) {
	if Verb(line) != "EPRT" {
		return "", false
	}
	body := args(line)
	if body == "" {
		return "", false
	}
	delim := body[0]
	fields := strings.Split(body, string(delim))
	// |1|host|port| 切出来是 ["", "1", "host", "port", ""]
	if len(fields) < 5 {
		return "", false
	}
	proto := strings.TrimSpace(fields[1])
	host := strings.TrimSpace(fields[2])
	portStr := strings.TrimSpace(fields[3])
	if proto != "1" && proto != "2" {
		return "", false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 || host == "" {
		return "", false
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), true
}

// SetEprt 把 EPRT 命令里的地址替换为 host:port，分隔符与协议号沿用报文里的
// IPv6 地址只有在客户端显式协商了 IPv6（proto=2）时才会带出，这里统一按
// 中继监听地址（IPv4）改写，因此 proto 恒为 1。
func SetEprt(line, host string, port int) (string, bool) {
	if port <= 0 || port > 65535 {
		return line, false
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || ip.To4() == nil {
		return line, false
	}
	body := strings.TrimRight(line, "\r\n")
	i := strings.IndexAny(body, " \t")
	if i < 0 {
		return line, false
	}
	rest := strings.TrimSpace(body[i+1:])
	if rest == "" {
		return line, false
	}
	delim := string(rest[0])
	return body[:i] + fmt.Sprintf(" %s1%s%s%s%d%s", delim, delim, ip.To4().String(), delim, port, delim) + eol(line), true
}

// IsAuthTls 判断是否为 AUTH TLS / AUTH SSL 命令。
// 这两条命令之后控制通道会升级为 TLS，明文改写不再可行。
func IsAuthTls(line string) bool {
	if Verb(line) != "AUTH" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(args(line))) {
	case "TLS", "SSL":
		return true
	}
	return false
}

// IsPasvCommand 判断客户端是否在要求被动模式，便于日志统计
func IsPasvCommand(line string) bool {
	switch Verb(line) {
	case "PASV", "EPSV":
		return true
	}
	return false
}

// ParsePortsSpec 解析端口范围配置，如 "40000-40010,40100"。
// 返回升序去重后的端口列表；格式非法的片段直接忽略。
func ParsePortsSpec(spec string) []int {
	var out []int
	seen := make(map[int]bool)
	add := func(p int) {
		if p <= 0 || p > 65535 || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, part := range strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		if i := strings.Index(part, "-"); i > 0 {
			lo, err1 := strconv.Atoi(strings.TrimSpace(part[:i]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(part[i+1:]))
			if err1 != nil || err2 != nil || lo > hi {
				continue
			}
			// 单个范围最多 4096 个端口，防止配置写错时生成巨大的切片
			if hi-lo > 4096 {
				hi = lo + 4096
			}
			for p := lo; p <= hi; p++ {
				add(p)
			}
			continue
		}
		if p, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			add(p)
		}
	}
	return out
}
