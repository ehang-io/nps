package lib

import (
	"encoding/base64"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const (
	COMPRESS_NONE_ENCODE = iota
	COMPRESS_NONE_DECODE
	COMPRESS_SNAPY_ENCODE
	COMPRESS_SNAPY_DECODE
	VERIFY_EER        = "vkey"
	WORK_MAIN         = "main"
	WORK_CHAN         = "chan"
	RES_SIGN          = "sign"
	RES_MSG           = "msg0"
	RES_CLOSE         = "clse"
	NEW_CONN          = "conn" //新连接标志
	CONN_SUCCESS      = "sucs"
	CONN_TCP          = "tcp"
	CONN_UDP          = "udp"
	UnauthorizedBytes = `HTTP/1.1 401 Unauthorized
Content-Type: text/plain; charset=utf-8
WWW-Authenticate: Basic realm="easyProxy"

401 Unauthorized`
	IO_EOF              = "PROXYEOF"
	ConnectionFailBytes = `HTTP/1.1 404 Not Found

`
)

//判断压缩方式
func GetCompressType(compress string) (int, int) {
	switch compress {
	case "":
		return COMPRESS_NONE_DECODE, COMPRESS_NONE_ENCODE
	case "snappy":
		return COMPRESS_SNAPY_DECODE, COMPRESS_SNAPY_ENCODE
	default:
		Fatalln("数据压缩格式错误")
	}
	return COMPRESS_NONE_DECODE, COMPRESS_NONE_ENCODE
}

//通过host获取对应的ip地址
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

//检查是否是域名
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

//检查basic认证
func CheckAuth(r *http.Request, user, passwd string) bool {
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 {
		return false
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
	i, _ := strconv.Atoi(str)
	return i
}

//简单的一个校验值
func Getverifyval(vkey string) string {
	return Md5(vkey)
}

func ChangeHostAndHeader(r *http.Request, host string, header string, addr string) {
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
	r.Header.Set("X-Forwarded-For", addr)
	r.Header.Set("X-Real-IP", addr)
}

func ReadAllFromFile(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
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

func GetRunPath() string {
	var path string
	if path = GetInstallPath(); !FileExists(path) {
		return "./"
	}
	return path
}
func GetInstallPath() string {
	var path string
	if IsWindows() {
		path = `C:\Program Files\nps`
	} else {
		path = "/etc/nps"
	}
	return path
}
func GetAppPath() string {
	if path, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		return path
	}
	return os.Args[0]
}
func IsWindows() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return false
}
func GetLogPath() string {
	var path string
	if IsWindows() {
		path = "./"
	} else {
		path = "/tmp"
	}
	return path
}
func GetPidPath() string {
	var path string
	if IsWindows() {
		path = "./"
	} else {
		path = "/tmp"
	}
	return path
}

func TestTcpPort(port int) bool {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), port, ""})
	defer l.Close()
	if err != nil {
		return false
	}
	return true
}
