package lib

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/snappy"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

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

func (s *CryptConn) Write(b []byte) (n int, err error) {
	n = len(b)
	if s.crypt {
		if b, err = AesEncrypt(b, []byte(cryptKey)); err != nil {
			return
		}
		if b, err = GetLenBytes(b); err != nil {
			return
		}
	}
	_, err = s.conn.Write(b)
	return
}

func (s *CryptConn) Read(b []byte) (n int, err error) {
	if s.crypt {
		var lens int
		var buf, bs []byte
		c := NewConn(s.conn)
		if lens, err = c.GetLen(); err != nil {
			return
		}
		if buf, err = c.ReadLen(lens); err != nil {
			return
		}
		if bs, err = AesDecrypt(buf, []byte(cryptKey)); err != nil {
			return
		}
		n = len(bs)
		copy(b, bs)
		return
	}
	return s.conn.Read(b)
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

func (s *SnappyConn) Read(b []byte) (n int, err error) {
	if n, err = s.r.Read(b); err != nil {
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

type GzipConn struct {
	w     *gzip.Writer
	r     *gzip.Reader
	crypt bool
}

func NewGzipConn(conn net.Conn, crypt bool) *GzipConn {
	c := new(GzipConn)
	c.crypt = crypt
	c.w = gzip.NewWriter(conn)
	c.r, err = gzip.NewReader(conn)
	fmt.Println("err", err)
	//错误处理
	return c
}

func (s *GzipConn) Write(b []byte) (n int, err error) {
	fmt.Println(string(b))
	if n, err = s.w.Write(b); err != nil {
		//err = s.w.Flush()
		//s.w.Close()
		return
	}
	err = s.w.Flush()
	return
}

func (s *GzipConn) Read(b []byte) (n int, err error) {
	fmt.Println("read")
	if n, err = s.r.Read(b); err != nil {
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
	conn net.Conn
}

func NewConn(conn net.Conn) *Conn {
	c := new(Conn)
	c.conn = conn
	return c
}

//读取指定内容长度
func (s *Conn) ReadLen(len int) ([]byte, error) {
	buf := make([]byte, len)
	if n, err := s.Read(buf); err != nil || n != len {
		return buf, errors.New("读取指定长度错误" + err.Error())
	}
	return buf, nil
}

//获取长度
func (s *Conn) GetLen() (int, error) {
	val := make([]byte, 4)
	if _, err := s.Read(val); err != nil {
		return 0, err
	}
	return GetLenByBytes(val)
}

//写入长度
func (s *Conn) WriteLen(buf []byte) (int, error) {
	var b []byte
	if b, err = GetLenBytes(buf); err != nil {
		return 0, err
	}
	return s.Write(b)
}

//读取flag
func (s *Conn) ReadFlag() (string, error) {
	val := make([]byte, 4)
	if _, err := s.Read(val); err != nil {
		return "", err
	}
	return string(val), err
}

//读取host 连接地址 压缩类型
func (s *Conn) GetHostFromConn() (typeStr string, host string, en, de int, crypt bool, err error) {
retry:
	ltype := make([]byte, 3)
	if _, err = s.Read(ltype); err != nil {
		return
	}
	if typeStr = string(ltype); typeStr == TEST_FLAG {
		en, de, crypt = s.GetConnInfoFromConn()
		goto retry
	}
	len, err := s.GetLen()
	if err != nil {
		return
	}
	hostByte := make([]byte, len)
	if _, err = s.Read(hostByte); err != nil {
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
	conn := s.conn.(*net.TCPConn)
	conn.SetReadDeadline(time.Time{})
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Duration(2 * time.Second))
}

//从tcp报文中解析出host
func (s *Conn) GetHost() (method, address string, rb []byte, err error, r *http.Request) {
	var b [32 * 1024]byte
	var n int
	if n, err = s.Read(b[:]); err != nil {
		return
	}
	rb = b[:n]
	r, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(rb)))
	if err != nil {
		log.Println("解析host出错：", err)
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

//压缩方式读
func (s *Conn) ReadFromCompress(b []byte, compress int) (int, error) {
	switch compress {
	case COMPRESS_SNAPY_DECODE:
		r := snappy.NewReader(s)
		return r.Read(b)
	default:
		return s.Read(b)
	}
	return 0, nil
}

//压缩方式写
func (s *Conn) WriteCompress(b []byte, compress int) (n int, err error) {
	switch compress {
	case COMPRESS_SNAPY_ENCODE:
		w := snappy.NewBufferedWriter(s)
		if n, err = w.Write(b); err == nil {
			w.Flush()
		}
		err = w.Close()
	default:
		n, err = s.Write(b)
	}
	return
}

//写压缩方式
func (s *Conn) WriteConnInfo(en, de int, crypt bool) {
	s.Write([]byte(strconv.Itoa(en) + strconv.Itoa(de) + GetStrByBool(crypt)))
}

//获取压缩方式
func (s *Conn) GetConnInfoFromConn() (en, de int, crypt bool) {
	buf := make([]byte, 3)
	s.Read(buf)
	en, _ = strconv.Atoi(string(buf[0]))
	de, _ = strconv.Atoi(string(buf[1]))
	crypt = GetBoolByStr(string(buf[2]))
	return
}

func (s *Conn) Close() error {
	return s.conn.Close()
}

func (s *Conn) Write(b []byte) (int, error) {
	return s.conn.Write(b)
}

func (s *Conn) Read(b []byte) (int, error) {
	return s.conn.Read(b)
}

func (s *Conn) wError() (int, error) {
	return s.Write([]byte(RES_MSG))
}

func (s *Conn) wSign() (int, error) {
	return s.Write([]byte(RES_SIGN))
}

func (s *Conn) wMain() (int, error) {
	return s.Write([]byte(WORK_MAIN))
}

func (s *Conn) wChan() (int, error) {
	return s.Write([]byte(WORK_CHAN))
}

func (s *Conn) wTest() (int, error) {
	return s.Write([]byte(TEST_FLAG))
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
