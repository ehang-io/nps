package lib

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/snappy"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SnappyConn struct {
	w *snappy.Writer
	r *snappy.Reader
}

func NewSnappyConn(conn net.Conn) *SnappyConn {
	c := new(SnappyConn)
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	return c
}

func (s *SnappyConn) Write(b []byte) (n int, err error) {
	if n, err = s.w.Write(b); err != nil {
		return
	}
	err = s.w.Flush()
	return
}

func (s *SnappyConn) Read(b []byte) (n int, err error) {
	return s.r.Read(b)
}

type GzipConn struct {
	w *gzip.Writer
	r *gzip.Reader
}

func NewGzipConn(conn net.Conn) *GzipConn {
	c := new(GzipConn)
	c.w = gzip.NewWriter(conn)
	c.r, err = gzip.NewReader(conn)
	return c
}

func (s *GzipConn) Write(b []byte) (n int, err error) {
	if n, err = s.w.Write(b); err != nil || err == io.EOF {
		err = s.w.Flush()
		s.w.Close()
		return
	}
	err = s.w.Flush()
	return
}

func (s *GzipConn) Read(b []byte) (n int, err error) {
	return s.r.Read(b)
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
	raw := make([]byte, 0)
	buff := make([]byte, 1024)
	c := 0
	for {
		clen, err := s.Read(buff)
		if err != nil && err != io.EOF {
			return raw, err
		}
		raw = append(raw, buff[:clen]...)
		if c += clen; c >= len {
			break
		}
	}
	if c != len {
		return raw, errors.New(fmt.Sprintf("已读取长度错误，已读取%dbyte，需要读取%dbyte。", c, len))
	}
	return raw, nil
}

//获取长度
func (s *Conn) GetLen() (int, error) {
	val := make([]byte, 4)
	_, err := s.Read(val)
	if err != nil {
		return 0, err
	}
	nlen := binary.LittleEndian.Uint32(val)
	if nlen <= 0 {
		return 0, errors.New("数据长度错误")
	}
	return int(nlen), nil
}

//写入长度
func (s *Conn) WriteLen(buf []byte) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	if err := binary.Write(raw, binary.LittleEndian, int32(len(buf))); err != nil {
		log.Println(err)
		return 0, err
	}
	if err = binary.Write(raw, binary.LittleEndian, buf); err != nil {
		log.Println(err)
		return 0, err
	}
	return s.Write(raw.Bytes())
}

//读取flag
func (s *Conn) ReadFlag() (string, error) {
	val := make([]byte, 4)
	_, err := s.Read(val)
	if err != nil {
		return "", err
	}
	return string(val), err
}

//读取host 连接地址 压缩类型
func (s *Conn) GetHostFromConn() (typeStr string, host string, en, de int, err error) {
retry:
	ltype := make([]byte, 3)
	if _, err = s.Read(ltype); err != nil {
		return
	}
	if typeStr = string(ltype); typeStr == TEST_FLAG {
		en, de = s.GetCompressTypeFromConn()
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
	case COMPRESS_GZIP_DECODE:
		r, err := gzip.NewReader(s)
		if err != nil {
			return 0, err
		}
		return r.Read(b)
	case COMPRESS_SNAPY_DECODE:
		r := snappy.NewReader(s)
		return r.Read(b)
	case COMPRESS_NONE:
		return s.Read(b)
	}
	return 0, nil
}

//压缩方式写
func (s *Conn) WriteCompress(b []byte, compress int) (n int, err error) {
	switch compress {
	case COMPRESS_GZIP_ENCODE:
		w := gzip.NewWriter(s)
		if n, err = w.Write(b); err == nil {
			w.Flush()
		}
		err = w.Close()
	case COMPRESS_SNAPY_ENCODE:
		w := snappy.NewBufferedWriter(s)
		if n, err = w.Write(b); err == nil {
			w.Flush()
		}
		err = w.Close()
	case COMPRESS_NONE:
		n, err = s.Write(b)
	}
	return
}

//写压缩方式
func (s *Conn) WriteCompressType(en, de int) {
	s.Write([]byte(strconv.Itoa(en) + strconv.Itoa(de)))
}

//获取压缩方式
func (s *Conn) GetCompressTypeFromConn() (en, de int) {
	buf := make([]byte, 2)
	s.Read(buf)
	en, _ = strconv.Atoi(string(buf[0]))
	de, _ = strconv.Atoi(string(buf[1]))
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
