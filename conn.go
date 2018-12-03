package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/snappy"
	"io"
	"log"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

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
		clen, err := s.conn.Read(buff)
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
	_, err := s.conn.Read(val)
	if err != nil {
		return "", err
	}
	return string(val), err
}

//读取host
func (s *Conn) GetHostFromConn() (typeStr string, host string, err error) {
	ltype := make([]byte, 3)
	_, err = s.Read(ltype)
	if err != nil {
		return
	}
	typeStr = string(ltype)
	len, err := s.GetLen()
	if err != nil {
		return
	}
	hostByte := make([]byte, len)
	_, err = s.conn.Read(hostByte)
	if err != nil {
		return
	}
	host = string(hostByte)
	return
}

//写tcp host
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
func (s *Conn) GetHost() (method, address string, rb []byte, err error) {
	var b [2048]byte
	var n int
	var host string
	if n, err = s.Read(b[:]); err != nil {
		return
	}
	rb = b[:n]
	//TODO：某些不规范报文可能会有问题
	fmt.Sscanf(string(b[:n]), "%s", &method)
	reg, err := regexp.Compile(`(\w+:\/\/)([^/:]+)(:\d*)?`)
	if err != nil {
		return
	}
	host = string(reg.Find(b[:]))
	hostPortURL, err := url.Parse(host)
	if err != nil {
		return
	}
	if hostPortURL.Opaque == "443" { //https访问
		address = hostPortURL.Scheme + ":443"
	} else { //http访问
		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			address = hostPortURL.Host + ":80"
		} else {
			address = hostPortURL.Host
		}
	}
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

func (s *Conn) WriteCompress(b []byte, compress int) (n int, err error) {
	switch compress {
	case COMPRESS_GZIP_ENCODE:
		w := gzip.NewWriter(s)
		if n, err = w.Write(b); err == nil {
			w.Flush()
		}
	case COMPRESS_SNAPY_ENCODE:
		w := snappy.NewBufferedWriter(s)
		if n, err = w.Write(b); err == nil {
			w.Flush()
		}
	case COMPRESS_NONE:
		n, err = s.Write(b)
	}
	return
}

func (s *Conn) wError() {
	s.conn.Write([]byte(RES_MSG))
}
func (s *Conn) wSign() {
	s.conn.Write([]byte(RES_SIGN))
}

func (s *Conn) wMain() {
	s.conn.Write([]byte(WORK_MAIN))
}

func (s *Conn) wChan() {
	s.conn.Write([]byte(WORK_CHAN))
}
