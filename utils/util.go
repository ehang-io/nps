package utils

import (
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	COMPRESS_NONE_ENCODE = iota
	COMPRESS_NONE_DECODE
	COMPRESS_SNAPY_ENCODE
	COMPRESS_SNAPY_DECODE
	VERIFY_EER         = "vkey"
	WORK_MAIN          = "main"
	WORK_CHAN          = "chan"
	RES_SIGN           = "sign"
	RES_MSG            = "msg0"
	CONN_SUCCESS       = "sucs"
	CONN_ERROR         = "fail"
	TEST_FLAG          = "tst"
	CONN_TCP           = "tcp"
	CONN_UDP           = "udp"
	Unauthorized_BYTES = `HTTP/1.1 401 Unauthorized
Content-Type: text/plain; charset=utf-8
WWW-Authenticate: Basic realm="easyProxy"

401 Unauthorized`
	IO_EOF = "PROXYEOF"
)

//copy
func Relay(in, out net.Conn, compressType int, crypt, mux bool) {
	switch compressType {
	case COMPRESS_SNAPY_ENCODE:
		copyBuffer(NewSnappyConn(in, crypt), out)
		if mux {
			out.Close()
			NewSnappyConn(in, crypt).Write([]byte(IO_EOF))
		}
	case COMPRESS_SNAPY_DECODE:
		copyBuffer(in, NewSnappyConn(out, crypt))
		if mux {
			in.Close()
		}
	case COMPRESS_NONE_ENCODE:
		copyBuffer(NewCryptConn(in, crypt), out)
		if mux {
			out.Close()
			NewCryptConn(in, crypt).Write([]byte(IO_EOF))
		}
	case COMPRESS_NONE_DECODE:
		copyBuffer(in, NewCryptConn(out, crypt))
		if mux {
			in.Close()
		}
	}
	if !mux {
		in.Close()
		out.Close()
	}
}

//判断压缩方式
func GetCompressType(compress string) (int, int) {
	switch compress {
	case "":
		return COMPRESS_NONE_DECODE, COMPRESS_NONE_ENCODE
	case "snappy":
		return COMPRESS_SNAPY_DECODE, COMPRESS_SNAPY_ENCODE
	default:
		log.Fatalln("数据压缩格式错误")
	}
	return COMPRESS_NONE_DECODE, COMPRESS_NONE_ENCODE
}

//通过host获取对应的ip地址
func Gethostbyname(hostname string) string {
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
func GetIntNoerrByStr(str string) int {
	i, _ := strconv.Atoi(str)
	return i
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 65535)
	},
}
// io.copy的优化版，读取buffer长度原为32*1024，与snappy不同，导致读取出的内容存在差异，不利于解密，特此修改
func copyBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	//TODO 回收问题
	buf := bufPool.Get().([]byte)
	for {
		nr, er := src.Read(buf)
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
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

//连接重置 清空缓存区
func FlushConn(c net.Conn) {
	c.SetReadDeadline(time.Now().Add(time.Second * 3))
	buf := bufPool.Get().([]byte)
	for {
		if _, err := c.Read(buf); err != nil {
			break
		}
	}
	c.SetReadDeadline(time.Time{})
}

//简单的一个校验值
func Getverifyval(vkey string) string {
	return Md5(vkey)
}
