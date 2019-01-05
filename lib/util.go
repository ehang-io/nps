package lib

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	disabledRedirect = errors.New("disabled redirect.")
)

const (
	COMPRESS_NONE_ENCODE = iota
	COMPRESS_NONE_DECODE
	COMPRESS_SNAPY_ENCODE
	COMPRESS_SNAPY_DECODE
	IO_EOF = "EOF"
)

//error
func BadRequest(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

//发送请求并转为bytes
func GetEncodeResponse(req *http.Request) ([]byte, error) {
	var respBytes []byte
	client := new(http.Client)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return disabledRedirect
	}
	resp, err := client.Do(req)
	disRedirect := err != nil && strings.Contains(err.Error(), disabledRedirect.Error())
	if err != nil && !disRedirect {
		return respBytes, err
	}
	if !disRedirect {
		defer resp.Body.Close()
	} else {
		resp.Body = nil
		resp.ContentLength = 0
	}
	respBytes, err = EncodeResponse(resp)
	return respBytes, nil
}

// 将request转为bytes
func EncodeRequest(r *http.Request) ([]byte, error) {
	raw := bytes.NewBuffer([]byte{})
	reqBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		return nil, err
	}
	binary.Write(raw, binary.LittleEndian, bool(r.URL.Scheme == "https"))
	binary.Write(raw, binary.LittleEndian, reqBytes)
	return raw.Bytes(), nil
}

// 将字节转为request
func DecodeRequest(data []byte) (*http.Request, error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data[1:])))
	if err != nil {
		return nil, err
	}
	str := strings.Split(req.Host, ":")
	req.Host, err = getHost(str[0])
	if err != nil {
		return nil, err
	}
	scheme := "http"
	if data[0] == 1 {
		scheme = "https"
	}
	req.URL, _ = url.Parse(fmt.Sprintf("%s://%s%s", scheme, req.Host, req.RequestURI))
	req.RequestURI = ""
	return req, nil
}

// 将response转为字节
func EncodeResponse(r *http.Response) ([]byte, error) {
	respBytes, err := httputil.DumpResponse(r, true)
	if err != nil {
		return nil, err
	}
	if config.Replace == 1 {
		respBytes = replaceHost(respBytes)
	}
	return respBytes, nil
}

// 将字节转为response
func DecodeResponse(data []byte) (*http.Response, error) {
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(data)), nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// 根据host地址从配置是文件中查找对应目标
func getHost(str string) (string, error) {
	for _, v := range config.SiteList {
		if v.Host == str {
			return v.Url + ":" + strconv.Itoa(v.Port), nil
		}
	}
	return "", errors.New("没有找到解析的的host!")
}

//替换
func replaceHost(resp []byte) []byte {
	str := string(resp)
	for _, v := range config.SiteList {
		str = strings.Replace(str, v.Url+":"+strconv.Itoa(v.Port), v.Host, -1)
		str = strings.Replace(str, v.Url, v.Host, -1)
	}
	return []byte(str)
}

//copy
func relay(in, out *Conn, compressType int, crypt, mux bool) {
	switch compressType {
	case COMPRESS_SNAPY_ENCODE:
		copyBuffer(NewSnappyConn(in.conn, crypt), out)
		if mux {
			NewSnappyConn(in.conn, crypt).Write([]byte(IO_EOF))
			out.Close()
		}
	case COMPRESS_SNAPY_DECODE:
		copyBuffer(in, NewSnappyConn(out.conn, crypt))
		if mux {
			in.Close()
		}
	case COMPRESS_NONE_ENCODE:
		copyBuffer(NewCryptConn(in.conn, crypt), out)
		if mux {
			NewCryptConn(in.conn, crypt).Write([]byte(IO_EOF))
			out.Close()
		}
	case COMPRESS_NONE_DECODE:
		copyBuffer(in, NewCryptConn(out.conn, crypt))
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
func getCompressType(compress string) (int, int) {
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

//简单的一个校验值
func getverifyval(vkey string) string {
	//单客户端模式
	if *verifyKey != "" {
		return Md5(*verifyKey)
	}
	return Md5(vkey)
}

//验证
func verify(verifyKeyMd5 string) bool {
	if *verifyKey != "" && getverifyval(*verifyKey) == verifyKeyMd5 {
		return true
	}
	if *verifyKey == "" {
		for k := range RunList {
			if getverifyval(k) == verifyKeyMd5 {
				return true
			}
		}
	}
	return false
}

//get key by host from x
func getKeyByHost(host string) (h *HostList, t *ServerConfig, err error) {
	for _, v := range CsvDb.Hosts {
		if strings.Contains(host, v.Host) {
			h = v
			t, err = CsvDb.GetTask(v.Vkey)
			return
		}
	}
	err = errors.New("未找到host对应的内网目标")
	return
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
func checkAuth(r *http.Request, user, passwd string) bool {
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
func GetIntNoerrByStr(str string) int {
	i, _ := strconv.Atoi(str)
	return i
}

// io.copy的优化版，读取buffer长度原为32*1024，与snappy不同，导致读取出的内容存在差异，不利于解密，特此修改
func copyBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}
	buf := make([]byte, 65535)
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
