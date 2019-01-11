package utils

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/golang/snappy"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const cryptKey = "1234567812345678"

type CryptConn struct {
	conn  net.Conn
	crypt bool
}

func NewCryptConn(conn net.Conn, crypt bool) *CryptConn {
	c := new(CryptConn)
	c.conn = conn
	c.crypt = crypt
	return c
}

//加密写
func (s *CryptConn) Write(b []byte) (n int, err error) {
	n = len(b)
	if s.crypt {
		if b, err = AesEncrypt(b, []byte(cryptKey)); err != nil {
			return
		}
	}
	if b, err = GetLenBytes(b); err != nil {
		return
	}
	_, err = s.conn.Write(b)
	return
}

//解密读
func (s *CryptConn) Read(b []byte) (n int, err error) {
	defer func() {
		if err == nil && n == len(IO_EOF) && string(b[:n]) == IO_EOF {
			err = io.EOF
			n = 0
		}
	}()
	var lens int
	var buf, bs []byte
	c := NewConn(s.conn)
	if lens, err = c.GetLen(); err != nil {
		return
	}
	if buf, err = c.ReadLen(lens); err != nil {
		return
	}
	if s.crypt {
		if bs, err = AesDecrypt(buf, []byte(cryptKey)); err != nil {
			return
		}
	} else {
		bs = buf
	}
	n = len(bs)
	copy(b, bs)
	return
}

type SnappyConn struct {
	w     *snappy.Writer
	r     *snappy.Reader
	crypt bool
}

func NewSnappyConn(conn net.Conn, crypt bool) *SnappyConn {
	c := new(SnappyConn)
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	c.crypt = crypt
	return c
}

//snappy压缩写 包含加密
func (s *SnappyConn) Write(b []byte) (n int, err error) {
	n = len(b)
	if s.crypt {
		if b, err = AesEncrypt(b, []byte(cryptKey)); err != nil {
			log.Println("encode crypt error:", err)
			return
		}
	}
	if _, err = s.w.Write(b); err != nil {
		return
	}
	err = s.w.Flush()
	return
}

//snappy压缩读 包含解密
func (s *SnappyConn) Read(b []byte) (n int, err error) {
	defer func() {
		if err == nil && n == len(IO_EOF) && string(b[:n]) == IO_EOF {
			err = io.EOF
			n = 0
		}
	}()
	if n, err = s.r.Read(b); err != nil || err == io.EOF {
		return
	}
	if s.crypt {
		var bs []byte
		if bs, err = AesDecrypt(b[:n], []byte(cryptKey)); err != nil {
			log.Println("decode crypt error:", err)
			return
		}
		n = len(bs)
		copy(b, bs)
	}
	return
}

type Conn struct {
	Conn net.Conn
}

//new conn
func NewConn(conn net.Conn) *Conn {
	c := new(Conn)
	c.Conn = conn
	return c
}

//读取指定长度内容
func (s *Conn) ReadLen(cLen int) ([]byte, error) {
	if cLen > 65536 {
		return nil, errors.New("长度错误")
	}
	buf := bufPool.Get().([]byte)[:cLen]
	if n, err := io.ReadFull(s, buf); err != nil || n != cLen {
		return buf, errors.New("读取指定长度错误" + err.Error())
	}
	return buf, nil
}

//获取长度
func (s *Conn) GetLen() (int, error) {
	val, err := s.ReadLen(4)
	if err != nil {
		return 0, err
	}
	return GetLenByBytes(val)
}

//写入长度+内容 粘包
func (s *Conn) WriteLen(buf []byte) (int, error) {
	var b []byte
	var err error
	if b, err = GetLenBytes(buf); err != nil {
		return 0, err
	}
	return s.Write(b)
}

//读取flag
func (s *Conn) ReadFlag() (string, error) {
	val, err := s.ReadLen(4)
	if err != nil {
		return "", err
	}
	return string(val), err
}

//读取host 连接地址 压缩类型
func (s *Conn) GetHostFromConn() (typeStr string, host string, en, de int, crypt, mux bool, err error) {
retry:
	lType, err := s.ReadLen(3)
	if err != nil {
		return
	}
	if typeStr = string(lType); typeStr == TEST_FLAG {
		en, de, crypt, mux = s.GetConnInfoFromConn()
		goto retry
	}
	cLen, err := s.GetLen()
	if err != nil {
		return
	}
	hostByte, err := s.ReadLen(cLen)
	if err != nil {
		return
	}
	host = string(hostByte)
	return
}

//写连接类型 和 host地址
func (s *Conn) WriteHost(ltype string, host string) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte(ltype))
	binary.Write(raw, binary.LittleEndian, int32(len([]byte(host))))
	binary.Write(raw, binary.LittleEndian, []byte(host))
	return s.Write(raw.Bytes())
}

//设置连接为长连接
func (s *Conn) SetAlive() {
	conn := s.Conn.(*net.TCPConn)
	conn.SetReadDeadline(time.Time{})
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Duration(2 * time.Second))
}

//从tcp报文中解析出host，连接类型等 TODO 多种情况
func (s *Conn) GetHost() (method, address string, rb []byte, err error, r *http.Request) {
	r, err = http.ReadRequest(bufio.NewReader(s))
	if err != nil {
		return
	}
	rb, err = httputil.DumpRequest(r, true)
	if err != nil {
		return
	}
	hostPortURL, err := url.Parse(r.Host)
	if err != nil {
		return
	}
	if hostPortURL.Opaque == "443" { //https访问
		address = r.Host + ":443"
	} else { //http访问
		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			address = r.Host + ":80"
		} else {
			address = r.Host
		}
	}
	return
}

//单独读（加密|压缩）
func (s *Conn) ReadFrom(b []byte, compress int, crypt bool) (int, error) {
	if COMPRESS_SNAPY_DECODE == compress {
		return NewSnappyConn(s.Conn, crypt).Read(b)
	}
	return NewCryptConn(s.Conn, crypt).Read(b)
}

//单独写（加密|压缩）
func (s *Conn) WriteTo(b []byte, compress int, crypt bool) (n int, err error) {
	if COMPRESS_SNAPY_ENCODE == compress {
		return NewSnappyConn(s.Conn, crypt).Write(b)
	}
	return NewCryptConn(s.Conn, crypt).Write(b)
}

//写压缩方式，加密
func (s *Conn) WriteConnInfo(en, de int, crypt, mux bool) {
	s.Write([]byte(strconv.Itoa(en) + strconv.Itoa(de) + GetStrByBool(crypt) + GetStrByBool(mux)))
}

//获取压缩方式，是否加密
func (s *Conn) GetConnInfoFromConn() (en, de int, crypt, mux bool) {
	buf, err := s.ReadLen(4)
	if err != nil {
		return
	}
	en, _ = strconv.Atoi(string(buf[0]))
	de, _ = strconv.Atoi(string(buf[1]))
	crypt = GetBoolByStr(string(buf[2]))
	mux = GetBoolByStr(string(buf[3]))
	return
}

//close
func (s *Conn) Close() error {
	return s.Conn.Close()
}

//write
func (s *Conn) Write(b []byte) (int, error) {
	return s.Conn.Write(b)
}

//read
func (s *Conn) Read(b []byte) (int, error) {
	return s.Conn.Read(b)
}

//write error
func (s *Conn) WriteError() (int, error) {
	return s.Write([]byte(RES_MSG))
}

//write sign flag
func (s *Conn) WriteSign() (int, error) {
	return s.Write([]byte(RES_SIGN))
}

//write main
func (s *Conn) WriteMain() (int, error) {
	return s.Write([]byte(WORK_MAIN))
}

//write chan
func (s *Conn) WriteChan() (int, error) {
	return s.Write([]byte(WORK_CHAN))
}

//write test
func (s *Conn) WriteTest() (int, error) {
	return s.Write([]byte(TEST_FLAG))
}

//write test
func (s *Conn) WriteSuccess() (int, error) {
	return s.Write([]byte(CONN_SUCCESS))
}

//write test
func (s *Conn) WriteFail() (int, error) {
	return s.Write([]byte(CONN_ERROR))
}

//获取长度+内容
func GetLenBytes(buf []byte) (b []byte, err error) {
	raw := bytes.NewBuffer([]byte{})
	if err = binary.Write(raw, binary.LittleEndian, int32(len(buf))); err != nil {
		return
	}
	if err = binary.Write(raw, binary.LittleEndian, buf); err != nil {
		return
	}
	b = raw.Bytes()
	return
}

//解析出长度
func GetLenByBytes(buf []byte) (int, error) {
	nlen := binary.LittleEndian.Uint32(buf)
	if nlen <= 0 {
		return 0, errors.New("数据长度错误")
	}
	return int(nlen), nil
}
