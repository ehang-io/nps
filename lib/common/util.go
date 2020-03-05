package common

import (
	"bytes"
	"ehang.io/nps/lib/version"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
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

//Check the legality of domain
func DomainCheck(domain string) bool {
	var match bool
	IsLine := "^((http://)|(https://))?([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)+[a-zA-Z]{2,6}(/)"
	NotLine := "^((http://)|(https://))?([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)+[a-zA-Z]{2,6}"
	match, _ = regexp.MatchString(IsLine, domain)
	if !match {
		match, _ = regexp.MatchString(NotLine, domain)
	}
	return match
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
	return pair[0] == user && pair[1] == passwd
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
func ChangeHostAndHeader(r *http.Request, host string, header string, addr string, addOrigin bool) {
	if host != "" {
		r.Host = host
	}
	if header != "" {
		h := strings.Split(header, "\n")
		for _, v := range h {
			hd := strings.Split(v, ":")
			if len(hd) == 2 {
				r.Header.Set(hd[0], hd[1])
			}
		}
	}
	addr = strings.Split(addr, ":")[0]
	if prior, ok := r.Header["X-Forwarded-For"]; ok {
		addr = strings.Join(prior, ", ") + ", " + addr
	}
	if addOrigin {
		r.Header.Set("X-Forwarded-For", addr)
		r.Header.Set("X-Real-IP", addr)
	}
}

//Read file content by file path
func ReadAllFromFile(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
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
	l, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), port, ""})
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
	l, err := net.ListenUDP("udp", &net.UDPAddr{net.ParseIP("0.0.0.0"), port, ""})
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
	if pi > 65536 || pi < 1 {
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
	arr := strings.Split(addr, ":")
	return arr[0]
}

//get port from the complete address
func GetPortByAddr(addr string) int {
	arr := strings.Split(addr, ":")
	if len(arr) < 2 {
		return 0
	}
	p, err := strconv.Atoi(arr[1])
	if err != nil {
		return 0
	}
	return p
}

func CopyBuffer(dst io.Writer, src io.Reader, label ...string) (written int64, err error) {
	buf := CopyBuff.Get()
	defer CopyBuff.Put(buf)
	for {
		nr, er := src.Read(buf)
		//if len(pr)>0 && pr[0] && nr > 50 {
		//	logs.Warn(string(buf[:50]))
		//}
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
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
func GeSynctMapLen(m sync.Map) int {
	var c int
	m.Range(func(key, value interface{}) bool {
		c++
		return true
	})
	return c
}

func GetExtFromPath(path string) string {
	s := strings.Split(path, ".")
	re, err := regexp.Compile(`(\w+)`)
	if err != nil {
		return ""
	}
	return string(re.Find([]byte(s[0])))
}

var externalIp string

func GetExternalIp() string {
	if externalIp != "" {
		return externalIp
	}
	resp, err := http.Get("http://myexternalip.com/raw")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	content, _ := ioutil.ReadAll(resp.Body)
	externalIp = string(content)
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
