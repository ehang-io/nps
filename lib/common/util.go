package common

import (
	"bytes"
	"crypto/subtle"
	"ehang.io/nps/lib/version"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"ehang.io/nps/lib/crypt"
)

//Get the corresponding IP address through domain name
func GetHostByName(hostname string) string {
	if !DomainCheck(hostname) {
		return hostname
	}
	ips, _ := net.LookupIP(hostname)
	if ips != nil {
		for _, v := range ips {
			if v.To4() != nil {
				return v.String()
			}
		}
	}
	return ""
}

//预编译的正则，避免每次调用 DomainCheck 都重新编译
var (
	domainLineRegexp   = regexp.MustCompile(`^((http://)|(https://))?([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}(/)`)
	domainNoLineRegexp = regexp.MustCompile(`^((http://)|(https://))?([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}`)
)

//Check the legality of domain
func DomainCheck(domain string) bool {
	return domainLineRegexp.MatchString(domain) || domainNoLineRegexp.MatchString(domain)
}

//Check if the Request request is validated
func CheckAuth(r *http.Request, user, passwd string) bool {
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 {
		s = strings.SplitN(r.Header.Get("Proxy-Authorization"), " ", 2)
		if len(s) != 2 {
			return false
		}
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return false
	}
	//常量时间比较，避免依据响应耗时逐字节猜测用户名/密码
	userOK := subtle.ConstantTimeCompare([]byte(pair[0]), []byte(user)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pair[1]), []byte(passwd)) == 1
	return userOK && passOK
}

//get bool by str
func GetBoolByStr(s string) bool {
	switch s {
	case "1", "true":
		return true
	}
	return false
}

//get str by bool
func GetStrByBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

//int
func GetIntNoErrByStr(str string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(str))
	return i
}

//Get verify value
func Getverifyval(vkey string) string {
	return crypt.Md5(vkey)
}

//Change headers and host of request
//maxForwardedForEntries 限制 X-Forwarded-For 链的最大长度。
//客户端可以自己伪造这个头，若不限制，后端会拿到一条无限增长的链，
//既有 header 放大风险，也会把链路上每一跳的内网地址一起泄露给业务端。
const maxForwardedForEntries = 16

func ChangeHostAndHeader(r *http.Request, host string, header string, addr string, addOrigin bool) {
	if host != "" {
		r.Host = host
	}
	if header != "" {
		h := strings.Split(header, "\n")
		for _, v := range h {
			//只按第一个冒号切分，避免 header 值本身包含冒号时被截断
			hd := strings.SplitN(v, ":", 2)
			if len(hd) == 2 {
				r.Header.Set(strings.TrimSpace(hd[0]), strings.TrimSpace(hd[1]))
			}
		}
	}
	clientIp := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		clientIp = h
	}
	clientIp = strings.Trim(clientIp, "[]")
	if !addOrigin {
		return
	}
	chain := clientIp
	if prior, ok := r.Header["X-Forwarded-For"]; ok {
		items := make([]string, 0, len(prior)+1)
		for _, v := range prior {
			for _, item := range strings.Split(v, ",") {
				if item = strings.TrimSpace(item); item != "" {
					items = append(items, item)
				}
			}
		}
		if len(items) > maxForwardedForEntries {
			items = items[len(items)-maxForwardedForEntries:]
		}
		items = append(items, clientIp)
		chain = strings.Join(items, ", ")
	}
	r.Header.Set("X-Forwarded-For", chain)
	r.Header.Set("X-Real-IP", clientIp)
}

//Read file content by file path
func ReadAllFromFile(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

//Judge whether the TCP port can open normally
func TestTcpPort(port int) bool {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: port})
	defer func() {
		if l != nil {
			l.Close()
		}
	}()
	if err != nil {
		return false
	}
	return true
}

//Judge whether the UDP port can open normally
func TestUdpPort(port int) bool {
	l, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: port})
	defer func() {
		if l != nil {
			l.Close()
		}
	}()
	if err != nil {
		return false
	}
	return true
}

//Write length and individual byte data
//Length prevents sticking
//# Characters are used to separate data
func BinaryWrite(raw *bytes.Buffer, v ...string) {
	b := GetWriteStr(v...)
	binary.Write(raw, binary.LittleEndian, int32(len(b)))
	binary.Write(raw, binary.LittleEndian, b)
}

// get seq str
func GetWriteStr(v ...string) []byte {
	buffer := new(bytes.Buffer)
	var l int32
	for _, v := range v {
		l += int32(len([]byte(v))) + int32(len([]byte(CONN_DATA_SEQ)))
		binary.Write(buffer, binary.LittleEndian, []byte(v))
		binary.Write(buffer, binary.LittleEndian, []byte(CONN_DATA_SEQ))
	}
	return buffer.Bytes()
}

//inArray str interface
func InStrArr(arr []string, val string) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

//inArray int interface
func InIntArr(arr []int, val int) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

//format ports str to a int array
func GetPorts(p string) []int {
	var ps []int
	arr := strings.Split(p, ",")
	for _, v := range arr {
		fw := strings.Split(v, "-")
		if len(fw) == 2 {
			if IsPort(fw[0]) && IsPort(fw[1]) {
				start, _ := strconv.Atoi(fw[0])
				end, _ := strconv.Atoi(fw[1])
				for i := start; i <= end; i++ {
					ps = append(ps, i)
				}
			} else {
				continue
			}
		} else if IsPort(v) {
			p, _ := strconv.Atoi(v)
			ps = append(ps, p)
		}
	}
	return ps
}

//is the string a port
func IsPort(p string) bool {
	pi, err := strconv.Atoi(p)
	if err != nil {
		return false
	}
	if pi > 65535 || pi < 1 {
		return false
	}
	return true
}

//if the s is just a port,return 127.0.0.1:s
func FormatAddress(s string) string {
	if strings.Contains(s, ":") {
		return s
	}
	return "127.0.0.1:" + s
}

//get address from the complete address
func GetIpByAddr(addr string) string {
	//优先按 host:port 解析，可正确处理 [::1]:8080 这类 IPv6 地址
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return strings.Trim(host, "[]")
	}
	if ip := net.ParseIP(strings.Trim(addr, "[]")); ip != nil {
		return ip.String()
	}
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

//get port from the complete address
func GetPortByAddr(addr string) int {
	if _, port, err := net.SplitHostPort(addr); err == nil {
		p, err := strconv.Atoi(port)
		if err != nil {
			return 0
		}
		return p
	}
	//纯 IP（含 IPv6）没有端口
	if net.ParseIP(strings.Trim(addr, "[]")) != nil {
		return 0
	}
	arr := strings.Split(addr, ":")
	if len(arr) < 2 {
		return 0
	}
	p, err := strconv.Atoi(arr[len(arr)-1])
	if err != nil {
		return 0
	}
	return p
}

func CopyBuffer(dst io.Writer, src io.Reader, label ...string) (written int64, err error) {
	return copyBuffer(dst, src, nil)
}

// CopyBufferTrack 与 CopyBuffer 相同，但在每次成功写入后回调 onWrite。
// 用于跟踪连接活跃度：一半连接已经 EOF、另一半还在传数据时，
// 必须靠“最近有没有流量”判断该继续等还是可以收工。
func CopyBufferTrack(dst io.Writer, src io.Reader, onWrite func(int)) (written int64, err error) {
	return copyBuffer(dst, src, onWrite)
}

func copyBuffer(dst io.Writer, src io.Reader, onWrite func(int)) (written int64, err error) {
	buf := CopyBuff.Get()
	defer CopyBuff.Put(buf)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if onWrite != nil {
					onWrite(nw)
				}
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

//send this ip forget to get a local udp port
func GetLocalUdpAddr() (net.Conn, error) {
	tmpConn, err := net.Dial("udp", "114.114.114.114:53")
	if err != nil {
		return nil, err
	}
	return tmpConn, tmpConn.Close()
}

//parse template
func ParseStr(str string) (string, error) {
	tmp := template.New("npc")
	var err error
	w := new(bytes.Buffer)
	if tmp, err = tmp.Parse(str); err != nil {
		return "", err
	}
	if err = tmp.Execute(w, GetEnvMap()); err != nil {
		return "", err
	}
	return w.String(), nil
}

//get env
func GetEnvMap() map[string]string {
	m := make(map[string]string)
	environ := os.Environ()
	for i := range environ {
		tmp := strings.Split(environ[i], "=")
		if len(tmp) == 2 {
			m[tmp[0]] = tmp[1]
		}
	}
	return m
}

//throw the empty element of the string array
func TrimArr(arr []string) []string {
	newArr := make([]string, 0)
	for _, v := range arr {
		if v != "" {
			newArr = append(newArr, v)
		}
	}
	return newArr
}

//
func IsArrContains(arr []string, val string) bool {
	if arr == nil {
		return false
	}
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

//remove value from string array
func RemoveArrVal(arr []string, val string) []string {
	for k, v := range arr {
		if v == val {
			arr = append(arr[:k], arr[k+1:]...)
			return arr
		}
	}
	return arr
}

//convert bytes to num
func BytesToNum(b []byte) int {
	var str string
	for i := 0; i < len(b); i++ {
		str += strconv.Itoa(int(b[i]))
	}
	x, _ := strconv.Atoi(str)
	return int(x)
}

//get the length of the sync map
func GeSynctMapLen(m *sync.Map) int {
	var c int
	m.Range(func(key, value interface{}) bool {
		c++
		return true
	})
	return c
}

func GetExtFromPath(path string) string {
	//原实现用正则取文件名首段，返回值并不是扩展名，这里改为取真实扩展名（小写，不含点）
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

var (
	externalIp     string
	externalIpLock sync.RWMutex
)

//SetExternalIp 由服务端读取配置后注入，GetExternalIp 不再自行探测
func SetExternalIp(ip string) {
	externalIpLock.Lock()
	externalIp = strings.TrimSpace(ip)
	externalIpLock.Unlock()
}

//GetExternalIp 返回配置里显式声明的公网地址。
//原实现每次都会去请求 http://myexternalip.com/raw：一方面把本机出口 IP 和
//“有人在这台机器上跑 nps”这两件事泄露给第三方站点，另一方面首次调用在锁内
//同步等 5 秒，会把请求全部堵住。现在只认配置（nps.conf 的 external_ip）。
func GetExternalIp() string {
	externalIpLock.RLock()
	defer externalIpLock.RUnlock()
	return externalIp
}

func GetIntranetIp() (error, string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, ""
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return nil, ipnet.IP.To4().String()
			}
		}
	}
	return errors.New("get intranet ip error"), ""
}

func IsPublicIP(IP net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch true {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return false
}

func GetServerIpByClientIp(clientIp net.IP) string {
	if IsPublicIP(clientIp) {
		return GetExternalIp()
	}
	_, ip := GetIntranetIp()
	return ip
}

func PrintVersion() {
	fmt.Printf("Version: %s\nCore version: %s\nSame core version of client and server can connect each other\n", version.VERSION, version.GetVersion())
}
