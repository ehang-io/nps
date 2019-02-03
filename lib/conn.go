package lib

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/golang/snappy"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const cryptKey = "1234567812345678"

type CryptConn struct {
	conn  net.Conn
	crypt bool
	rate  *Rate
}

func NewCryptConn(conn net.Conn, crypt bool, rate *Rate) *CryptConn {
	c := new(CryptConn)
	c.conn = conn
	c.crypt = crypt
	c.rate = rate
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
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}

//解密读
func (s *CryptConn) Read(b []byte) (n int, err error) {
	var lens int
	var buf []byte
	var rb []byte
	c := NewConn(s.conn)
	if lens, err = c.GetLen(); err != nil {
		return
	}
	if buf, err = c.ReadLen(lens); err != nil {
		return
	}
	if s.crypt {
		if rb, err = AesDecrypt(buf, []byte(cryptKey)); err != nil {
			return
		}
	} else {
		rb = buf
	}
	copy(b, rb)
	n = len(rb)
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}

type SnappyConn struct {
	w     *snappy.Writer
	r     *snappy.Reader
	crypt bool
	rate  *Rate
}

func NewSnappyConn(conn net.Conn, crypt bool, rate *Rate) *SnappyConn {
	c := new(SnappyConn)
	c.w = snappy.NewBufferedWriter(conn)
	c.r = snappy.NewReader(conn)
	c.crypt = crypt
	c.rate = rate
	return c
}

//snappy压缩写 包含加密
func (s *SnappyConn) Write(b []byte) (n int, err error) {
	n = len(b)
	if s.crypt {
		if b, err = AesEncrypt(b, []byte(cryptKey)); err != nil {
			Println("encode crypt error:", err)
			return
		}
	}
	if _, err = s.w.Write(b); err != nil {
		return
	}
	if err = s.w.Flush(); err != nil {
		return
	}
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}

//snappy压缩读 包含解密
func (s *SnappyConn) Read(b []byte) (n int, err error) {
	buf := BufPool.Get().([]byte)
	defer BufPool.Put(buf)
	if n, err = s.r.Read(buf); err != nil {
		return
	}
	var bs []byte
	if s.crypt {
		if bs, err = AesDecrypt(buf[:n], []byte(cryptKey)); err != nil {
			Println("decode crypt error:", err)
			return
		}
	} else {
		bs = buf[:n]
	}
	n = len(bs)
	copy(b, bs)
	if s.rate != nil {
		s.rate.Get(int64(n))
	}
	return
}

type Conn struct {
	Conn net.Conn
	sync.Mutex
}

//new conn
func NewConn(conn net.Conn) *Conn {
	c := new(Conn)
	c.Conn = conn
	return c
}

//从tcp报文中解析出host，连接类型等
func (s *Conn) GetHost() (method, address string, rb []byte, err error, r *http.Request) {
	var b [32 * 1024]byte
	var n int
	if n, err = s.Read(b[:]); err != nil {
		return
	}
	rb = b[:n]
	r, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(rb)))
	if err != nil {
		return
	}
	hostPortURL, err := url.Parse(r.Host)
	if err != nil {
		address = r.Host
		err = nil
		return
	}
	if hostPortURL.Opaque == "443" { //https访问
		if strings.Index(r.Host, ":") == -1 { //host不带端口， 默认80
			address = r.Host + ":443"
		} else {
			address = r.Host
		}
	} else { //http访问
		if strings.Index(r.Host, ":") == -1 { //host不带端口， 默认80
			address = r.Host + ":80"
		} else {
			address = r.Host
		}
	}
	return
}

//读取指定长度内容
func (s *Conn) ReadLen(cLen int) ([]byte, error) {
	if cLen > poolSize {
		return nil, errors.New("长度错误" + strconv.Itoa(cLen))
	}
	var buf []byte
	if cLen <= poolSizeSmall {
		buf = BufPoolSmall.Get().([]byte)[:cLen]
		defer BufPoolSmall.Put(buf)
	} else {
		buf = BufPoolMax.Get().([]byte)[:cLen]
		defer BufPoolMax.Put(buf)
	}
	if n, err := io.ReadFull(s, buf); err != nil || n != cLen {
		return buf, errors.New("读取指定长度错误" + err.Error())
	}
	return buf, nil
}

//read length or id (content length=4)
func (s *Conn) GetLen() (int, error) {
	val, err := s.ReadLen(4)
	if err != nil {
		return 0, err
	}
	return GetLenByBytes(val)
}

//read flag
func (s *Conn) ReadFlag() (string, error) {
	val, err := s.ReadLen(4)
	if err != nil {
		return "", err
	}
	return string(val), err
}

//read connect status
func (s *Conn) GetConnStatus() (id int, status bool, err error) {
	id, err = s.GetLen()
	if err != nil {
		return
	}
	var b []byte
	if b, err = s.ReadLen(1); err != nil {
		return
	} else {
		status = GetBoolByStr(string(b[0]))
	}
	return
}

//设置连接为长连接
func (s *Conn) SetAlive() {
	conn := s.Conn.(*net.TCPConn)
	conn.SetReadDeadline(time.Time{})
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Duration(2 * time.Second))
}

//set read dead time
func (s *Conn) SetReadDeadline(t time.Duration) {
	s.Conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(t) * time.Second))
}

//单独读（加密|压缩）
func (s *Conn) ReadFrom(b []byte, compress int, crypt bool, rate *Rate) (int, error) {
	if COMPRESS_SNAPY_DECODE == compress {
		return NewSnappyConn(s.Conn, crypt, rate).Read(b)
	}
	return NewCryptConn(s.Conn, crypt, rate).Read(b)
}

//单独写（加密|压缩）
func (s *Conn) WriteTo(b []byte, compress int, crypt bool, rate *Rate) (n int, err error) {
	if COMPRESS_SNAPY_ENCODE == compress {
		return NewSnappyConn(s.Conn, crypt, rate).Write(b)
	}
	return NewCryptConn(s.Conn, crypt, rate).Write(b)
}

//send msg
func (s *Conn) SendMsg(content []byte, link *Link) (n int, err error) {
	/*
		The msg info is formed as follows:
		+----+--------+
		|id | content |
		+----+--------+
		| 4  |  ...   |
		+----+--------+
*/
	s.Lock()
	defer s.Unlock()
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, int32(link.Id))
	if n, err = s.Write(raw.Bytes()); err != nil {
		return
	}
	raw.Reset()
	binary.Write(raw, binary.LittleEndian, content)
	n, err = s.WriteTo(raw.Bytes(), link.En, link.Crypt, link.Rate)
	return
}

//get msg content from conn
func (s *Conn) GetMsgContent(link *Link) (content []byte, err error) {
	s.Lock()
	defer s.Unlock()
	buf := BufPoolCopy.Get().([]byte)
	if n, err := s.ReadFrom(buf, link.De, link.Crypt, link.Rate); err == nil && n > 4 {
		content = buf[:n]
	}
	return
}

//send info for link
func (s *Conn) SendLinkInfo(link *Link) (int, error) {
	/*
		The  link info is formed as follows:
		+----------+------+----------+------+----------+-----+
		| id | len | type |  hostlen | host | en | de |crypt |
		+----------+------+----------+------+---------+------+
		| 4  |  4  |  3   |     4    | host | 1  | 1  |   1  |
		+----------+------+----------+------+----+----+------+
	*/
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte(NEW_CONN))
	binary.Write(raw, binary.LittleEndian, int32(14+len(link.Host)))
	binary.Write(raw, binary.LittleEndian, int32(link.Id))
	binary.Write(raw, binary.LittleEndian, []byte(link.ConnType))
	binary.Write(raw, binary.LittleEndian, int32(len(link.Host)))
	binary.Write(raw, binary.LittleEndian, []byte(link.Host))
	binary.Write(raw, binary.LittleEndian, []byte(strconv.Itoa(link.En)))
	binary.Write(raw, binary.LittleEndian, []byte(strconv.Itoa(link.De)))
	binary.Write(raw, binary.LittleEndian, []byte(GetStrByBool(link.Crypt)))
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
}

func (s *Conn) GetLinkInfo() (link *Link, err error) {
	s.Lock()
	defer s.Unlock()
	var hostLen, n int
	var buf []byte
	if n, err = s.GetLen(); err != nil {
		return
	}
	link = new(Link)
	if buf, err = s.ReadLen(n); err != nil {
		return
	}
	if link.Id, err = GetLenByBytes(buf[:4]); err != nil {
		return
	}
	link.ConnType = string(buf[4:7])
	if hostLen, err = GetLenByBytes(buf[7:11]); err != nil {
		return
	} else {
		link.Host = string(buf[11 : 11+hostLen])
		link.En = GetIntNoErrByStr(string(buf[11+hostLen]))
		link.De = GetIntNoErrByStr(string(buf[12+hostLen]))
		link.Crypt = GetBoolByStr(string(buf[13+hostLen]))
	}
	return
}

//write connect success
func (s *Conn) WriteSuccess(id int) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, int32(id))
	binary.Write(raw, binary.LittleEndian, []byte("1"))
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
}

//write connect fail
func (s *Conn) WriteFail(id int) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, int32(id))
	binary.Write(raw, binary.LittleEndian, []byte("0"))
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
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

//write sign flag
func (s *Conn) WriteClose() (int, error) {
	return s.Write([]byte(RES_CLOSE))
}

//write main
func (s *Conn) WriteMain() (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.Write([]byte(WORK_MAIN))
}

//write chan
func (s *Conn) WriteChan() (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.Write([]byte(WORK_CHAN))
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
