package proxy

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

const (
	// tcpAuthTimeout 认证/TLS 握手的读超时，避免半开连接长期占用协程
	tcpAuthTimeout = 10 * time.Second
	// tcpAuthMaxLine 单行认证信息的最大长度
	tcpAuthMaxLine = 8 * 1024
	// tcpAuthMaxHeader HTTP 认证头部的最大总长度
	tcpAuthMaxHeader = 16 * 1024

	authOKBytes          = "AUTH OK\r\n"
	authFailedBytes      = "AUTH FAILED\r\n"
	authChallengeBytes   = "HTTP/1.1 401 Unauthorized\r\nWWW-Authenticate: Basic realm=\"nps\"\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	connectEstablishedB  = "HTTP/1.1 200 Connection established\r\n\r\n"
	authHeaderName       = "authorization"
	proxyAuthHeaderName  = "proxy-authorization"
	authSchemeBasic      = "basic"
	methodConnect        = "CONNECT"
	tlsHandshakeAuthKind = "tls"
)

var (
	errAuthFailed   = errors.New("tcp tunnel authentication failed")
	errAuthTooLarge = errors.New("tcp tunnel authentication frame too large")
	// errAuthResponded 表示失败响应已经写回给访问者并正常关闭，
	// 调用方不要再发 RST，否则客户端可能收不到 401 / AUTH FAILED。
	errAuthResponded = errors.New("tcp tunnel auth response sent, connection closed")
)

// wrapTunnelConn 按隧道配置包装访问者连接：先 TLS（可选），再 Basic 认证（可选）。
// 认证帧会在建立隧道之前被消费掉，剩余字节通过 preReadConn 回放给内网目标，
// 因此未通过认证的连接既拿不到服务指纹，也不会被转发到内网。
func wrapTunnelConn(c net.Conn, task *file.Tunnel) (net.Conn, error) {
	if task == nil || task.Mode != "tcp" {
		return c, nil
	}
	if task.TlsEnabled() {
		certFile, keyFile := tunnelTlsFiles(task)
		wrapped, err := crypt.WrapServerTls(c, certFile, keyFile)
		if err != nil {
			return nil, err
		}
		_ = c.SetDeadline(time.Now().Add(tcpAuthTimeout))
		if err = wrapped.(*tls.Conn).Handshake(); err != nil {
			_ = c.SetDeadline(time.Time{})
			return nil, err
		}
		_ = c.SetDeadline(time.Time{})
		c = wrapped
	}
	if !task.AuthEnabled() {
		return c, nil
	}
	return tcpAuthHandshake(c, task.AuthUser, task.AuthPass)
}

// tunnelTlsFiles 返回隧道 TLS 使用的证书对，隧道未配置时回退到 nps.conf 的全局证书。
// 两者都为空时返回空字符串，由 crypt 使用内建自签证书。
func tunnelTlsFiles(task *file.Tunnel) (string, string) {
	certFile := strings.TrimSpace(task.TlsCertFile)
	keyFile := strings.TrimSpace(task.TlsKeyFile)
	if certFile == "" {
		certFile = strings.TrimSpace(beego.AppConfig.DefaultString("bridge_cert_file", ""))
	}
	if keyFile == "" {
		keyFile = strings.TrimSpace(beego.AppConfig.DefaultString("bridge_key_file", ""))
	}
	if certFile == "" || keyFile == "" {
		return "", ""
	}
	return certFile, keyFile
}

// tcpAuthHandshake 在隧道上做一次 Basic 认证，返回的 conn 会把已读出的多余数据回放出去。
// 支持三种入口，覆盖浏览器、命令行与 RDP 类客户端：
//  1. HTTP CONNECT 代理：CONNECT host:port HTTP/1.1 + Proxy-Authorization: Basic xxx
//  2. 普通 HTTP 请求：GET / HTTP/1.1 + Authorization: Basic xxx（认证头会被剥离）
//  3. 行协议：AUTH Basic xxx
func tcpAuthHandshake(c net.Conn, user, pass string) (net.Conn, error) {
	br := bufio.NewReaderSize(c, tcpAuthMaxLine)
	_ = c.SetReadDeadline(time.Now().Add(tcpAuthTimeout))
	defer c.SetReadDeadline(time.Time{})

	line, err := readAuthLine(br)
	if err != nil {
		return nil, err
	}
	if isHttpRequestLine(line) {
		return httpAuthHandshake(c, br, line, user, pass)
	}
	cred, ok := parseAuthLine(line)
	if !ok {
		// 既不是 HTTP 请求也不是认证帧：按扫描器处理，不回任何字节
		return nil, errAuthFailed
	}
	if !checkBasicCredential(cred, user, pass) {
		logReject(c.RemoteAddr(), "tcp tunnel auth failed")
		_, _ = c.Write([]byte(authFailedBytes))
		_ = c.Close()
		return nil, errAuthResponded
	}
	if _, err = c.Write([]byte(authOKBytes)); err != nil {
		return nil, err
	}
	logs.Trace("tcp tunnel auth passed, remote address %s", c.RemoteAddr())
	return newPreReadConn(c, nil, br), nil
}

// httpAuthHandshake 处理 HTTP 形态的认证入口
func httpAuthHandshake(c net.Conn, br *bufio.Reader, reqLine, user, pass string) (net.Conn, error) {
	header, err := readHttpHeader(br)
	if err != nil {
		return nil, err
	}
	cred := findBasicCredential(header)
	if cred == "" || !checkBasicCredential(cred, user, pass) {
		logReject(c.RemoteAddr(), "tcp tunnel auth failed")
		_, _ = c.Write([]byte(authChallengeBytes))
		_ = c.Close()
		return nil, errAuthResponded
	}
	if method, _ := splitRequestLine(reqLine); method == methodConnect {
		// 认证通过，按 HTTP 代理语义回 200，之后是纯 TCP 转发
		if _, err = c.Write([]byte(connectEstablishedB)); err != nil {
			return nil, err
		}
		logs.Trace("tcp tunnel auth passed via CONNECT, remote address %s", c.RemoteAddr())
		return newPreReadConn(c, nil, br), nil
	}
	logs.Trace("tcp tunnel auth passed via http request, remote address %s", c.RemoteAddr())
	return newPreReadConn(c, stripAuthHeader(reqLine, header), br), nil
}

// preReadConn 先回放已读出的字节，再继续从底层连接读取。
// br 中尚未消费的缓冲同样通过 io.MultiReader 正确接续。
type preReadConn struct {
	net.Conn
	r io.Reader
}

func (c *preReadConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func newPreReadConn(c net.Conn, replay []byte, br *bufio.Reader) net.Conn {
	readers := make([]io.Reader, 0, 2)
	if len(replay) > 0 {
		readers = append(readers, bytes.NewReader(replay))
	}
	if br != nil && br.Buffered() > 0 {
		readers = append(readers, br)
	}
	if len(readers) == 0 {
		return c
	}
	return &preReadConn{Conn: c, r: io.MultiReader(readers...)}
}

// readAuthLine 读取一行认证信息，超过缓冲上限直接判为非法输入
func readAuthLine(br *bufio.Reader) (string, error) {
	line, err := br.ReadSlice('\n')
	if err == bufio.ErrBufferFull {
		return "", errAuthTooLarge
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(line), "\r\n"), nil
}

// readHttpHeader 读完 HTTP 头部（不含空行），带总长度上限
func readHttpHeader(br *bufio.Reader) ([]string, error) {
	var (
		lines []string
		total int
	)
	for {
		line, err := readAuthLine(br)
		if err != nil {
			return nil, err
		}
		if line == "" {
			return lines, nil
		}
		total += len(line) + 2
		if total > tcpAuthMaxHeader {
			return nil, errAuthTooLarge
		}
		lines = append(lines, line)
	}
}

// findBasicCredential 从 HTTP 头里找到 Basic 凭据（base64 部分）
func findBasicCredential(header []string) string {
	for _, line := range header {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name != authHeaderName && name != proxyAuthHeaderName {
			continue
		}
		if cred, ok := parseBasicScheme(value); ok {
			return cred
		}
	}
	return ""
}

// stripAuthHeader 重建请求行与头部，去掉认证头后再回放给内网服务
func stripAuthHeader(reqLine string, header []string) []byte {
	var b bytes.Buffer
	b.WriteString(reqLine)
	b.WriteString("\r\n")
	for _, line := range header {
		name, _, ok := strings.Cut(line, ":")
		if ok {
			lname := strings.ToLower(strings.TrimSpace(name))
			if lname == authHeaderName || lname == proxyAuthHeaderName {
				continue
			}
		}
		b.WriteString(line)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

// parseAuthLine 解析 "AUTH Basic xxx" / "Authorization: Basic xxx" 形式的首行
func parseAuthLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if name, value, ok := strings.Cut(trimmed, ":"); ok {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case authHeaderName, proxyAuthHeaderName:
			return parseBasicScheme(value)
		}
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 3 && strings.EqualFold(fields[0], "AUTH") && strings.EqualFold(fields[1], authSchemeBasic) {
		return fields[2], true
	}
	return "", false
}

func parseBasicScheme(value string) (string, bool) {
	fields := strings.Fields(value)
	if len(fields) != 2 || !strings.EqualFold(fields[0], authSchemeBasic) {
		return "", false
	}
	return fields[1], true
}

// checkBasicCredential 校验 base64 编码的 user:pass，使用常量时间比较避免时序侧信道
func checkBasicCredential(raw, user, pass string) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	u, p, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return false
	}
	userOk := subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1
	passOk := subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
	return userOk && passOk
}

// isHttpRequestLine 判断首行是否是 HTTP 请求行
func isHttpRequestLine(line string) bool {
	if !strings.Contains(line, "HTTP/") {
		return false
	}
	method, _ := splitRequestLine(line)
	switch method {
	case "GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH", "TRACE", methodConnect:
		return true
	}
	return false
}

func splitRequestLine(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	return fields[0], fields[1]
}
