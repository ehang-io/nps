package lib

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	disabledRedirect = errors.New("disabled redirect.")
)

const (
	COMPRESS_NONE = iota
	COMPRESS_SNAPY_ENCODE
	COMPRESS_SNAPY_DECODE
	COMPRESS_GZIP_ENCODE
	COMPRESS_GZIP_DECODE
)

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

//// 将response转为字节
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

func getHost(str string) (string, error) {
	for _, v := range config.SiteList {
		if v.Host == str {
			return v.Url + ":" + strconv.Itoa(v.Port), nil
		}
	}
	return "", errors.New("没有找到解析的的host!")
}

func replaceHost(resp []byte) []byte {
	str := string(resp)
	for _, v := range config.SiteList {
		str = strings.Replace(str, v.Url+":"+strconv.Itoa(v.Port), v.Host, -1)
		str = strings.Replace(str, v.Url, v.Host, -1)
	}
	return []byte(str)
}

func relay(in, out *Conn, compressType int) {
	buf := make([]byte, 32*1024)
	switch compressType {
	case COMPRESS_GZIP_ENCODE:
		//TODO:GZIP压缩存在问题有待解决
		w := gzip.NewWriter(in)
		for {
			n, err := out.Read(buf)
			if err != nil || err == io.EOF {
				break
			}
			if _, err = w.Write(buf[:n]); err != nil {
				break
			}
			if err = w.Flush(); err != nil {
				log.Println(err)
				break
			}
		}
		w.Close()
	case COMPRESS_SNAPY_ENCODE:
		io.Copy(NewSnappyConn(in.conn), out)
	case COMPRESS_GZIP_DECODE:
		io.Copy(in, NewGzipConn(out.conn))
	case COMPRESS_SNAPY_DECODE:
		io.Copy(in, NewSnappyConn(out.conn))
	default:
		io.Copy(in, out)
	}
	out.Close()
	in.Close()
}

type Site struct {
	Host string
	Url  string
	Port int
}
type Config struct {
	SiteList []Site
	Replace  int
}
type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}
func (jst *JsonStruct) Load(filename string) (Config, error) {
	data, err := ioutil.ReadFile(filename)
	config := Config{}
	if err != nil {
		return config, errors.New("配置文件打开错误")
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, errors.New("配置文件解析错误")
	}
	return config, nil
}

//判断压缩方式
func getCompressType(compress string) (int, int) {
	switch compress {
	case "":
		return COMPRESS_NONE, COMPRESS_NONE
	case "gzip":
		return COMPRESS_GZIP_DECODE, COMPRESS_GZIP_ENCODE
	case "snappy":
		return COMPRESS_SNAPY_DECODE, COMPRESS_SNAPY_ENCODE
	default:
		log.Fatalln("数据压缩格式错误")
	}
	return COMPRESS_NONE, COMPRESS_NONE
}

// 简单的一个校验值
func getverifyval(vkey string) string {
	//单客户端模式
	if *verifyKey != "" {
		return Md5(*verifyKey)
	}
	return Md5(vkey)
}

func verify(verifyKeyMd5 string) bool {
	if getverifyval(*verifyKey) == verifyKeyMd5 {
		return true
	}
	if *verifyKey == "" {
		for _, v := range CsvDb.Tasks {
			if _, ok := RunList[v.VerifyKey]; getverifyval(v.VerifyKey) == verifyKeyMd5 && ok {
				return true
			}
		}
	}
	return false
}

func getKeyByHost(host string) (h *HostList, t *TaskList, err error) {
	for _, v := range CsvDb.Hosts {
		if strings.Contains(host, v.Host) {
			h = v
			t, err = CsvDb.GetTask(v.Vkey)
			if err != nil {
				return
			}
			return
		}
	}
	err = errors.New("未找到host对应的内网目标")
	return
}

//生成32位md5字串
func Md5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

//生成随机验证密钥
func GetRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

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
