package conn

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/lib/rate"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
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
	if cLen > pool.PoolSize {
		return nil, errors.New("长度错误" + strconv.Itoa(cLen))
	}
	var buf []byte
	if cLen <= pool.PoolSizeSmall {
		buf = pool.BufPoolSmall.Get().([]byte)[:cLen]
		defer pool.BufPoolSmall.Put(buf)
	} else {
		buf = pool.BufPoolMax.Get().([]byte)[:cLen]
		defer pool.BufPoolMax.Put(buf)
	}
	if n, err := io.ReadFull(s, buf); err != nil || n != cLen {
		return buf, errors.New("Error reading specified length " + err.Error())
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
		status = common.GetBoolByStr(string(b[0]))
	}
	return
}

//设置连接为长连接
func (s *Conn) SetAlive(tp string) {
	if tp == "kcp" {
		s.setKcpAlive()
	} else {
		s.setTcpAlive()
	}
}

//设置连接为长连接
func (s *Conn) setTcpAlive() {
	conn := s.Conn.(*net.TCPConn)
	conn.SetReadDeadline(time.Time{})
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(time.Duration(2 * time.Second))
}

//设置连接为长连接
func (s *Conn) setKcpAlive() {
	conn := s.Conn.(*kcp.UDPSession)
	conn.SetReadDeadline(time.Time{})
}

//设置连接为长连接
func (s *Conn) SetReadDeadline(t time.Duration, tp string) {
	if tp == "kcp" {
		s.SetKcpReadDeadline(t)
	} else {
		s.SetTcpReadDeadline(t)
	}
}

//set read dead time
func (s *Conn) SetTcpReadDeadline(t time.Duration) {
	s.Conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(t) * time.Second))
}

//set read dead time
func (s *Conn) SetKcpReadDeadline(t time.Duration) {
	s.Conn.(*kcp.UDPSession).SetReadDeadline(time.Now().Add(time.Duration(t) * time.Second))
}

//单独读（加密|压缩）
func (s *Conn) ReadFrom(b []byte, compress int, crypt bool, rate *rate.Rate) (int, error) {
	if common.COMPRESS_SNAPY_DECODE == compress {
		return NewSnappyConn(s.Conn, crypt, rate).Read(b)
	}
	return NewCryptConn(s.Conn, crypt, rate).Read(b)
}

//单独写（加密|压缩）
func (s *Conn) WriteTo(b []byte, compress int, crypt bool, rate *rate.Rate) (n int, err error) {
	if common.COMPRESS_SNAPY_ENCODE == compress {
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
	buf := pool.BufPoolCopy.Get().([]byte)
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
	binary.Write(raw, binary.LittleEndian, []byte(common.NEW_CONN))
	binary.Write(raw, binary.LittleEndian, int32(14+len(link.Host)))
	binary.Write(raw, binary.LittleEndian, int32(link.Id))
	binary.Write(raw, binary.LittleEndian, []byte(link.ConnType))
	binary.Write(raw, binary.LittleEndian, int32(len(link.Host)))
	binary.Write(raw, binary.LittleEndian, []byte(link.Host))
	binary.Write(raw, binary.LittleEndian, []byte(strconv.Itoa(link.En)))
	binary.Write(raw, binary.LittleEndian, []byte(strconv.Itoa(link.De)))
	binary.Write(raw, binary.LittleEndian, []byte(common.GetStrByBool(link.Crypt)))
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
}

func (s *Conn) GetLinkInfo() (lk *Link, err error) {
	s.Lock()
	defer s.Unlock()
	var hostLen, n int
	var buf []byte
	if n, err = s.GetLen(); err != nil {
		return
	}
	lk = new(Link)
	if buf, err = s.ReadLen(n); err != nil {
		return
	}
	if lk.Id, err = GetLenByBytes(buf[:4]); err != nil {
		return
	}
	lk.ConnType = string(buf[4:7])
	if hostLen, err = GetLenByBytes(buf[7:11]); err != nil {
		return
	} else {
		lk.Host = string(buf[11 : 11+hostLen])
		lk.En = common.GetIntNoErrByStr(string(buf[11+hostLen]))
		lk.De = common.GetIntNoErrByStr(string(buf[12+hostLen]))
		lk.Crypt = common.GetBoolByStr(string(buf[13+hostLen]))
	}
	return
}

//send host info
func (s *Conn) SendHostInfo(h *file.Host) (int, error) {
	/*
		The task info is formed as follows:
		+----+-----+---------+
		|type| len | content |
		+----+---------------+
		| 4  |  4  |   ...   |
		+----+---------------+
	*/
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte(common.NEW_HOST))
	common.BinaryWrite(raw, h.Host, h.Target, h.HeaderChange, h.HostChange, h.Remark, h.Location)
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
}

func (s *Conn) GetAddStatus() (b bool) {
	binary.Read(s.Conn, binary.LittleEndian, &b)
	return
}

func (s *Conn) WriteAddOk() error {
	return binary.Write(s.Conn, binary.LittleEndian, true)
}

func (s *Conn) WriteAddFail() error {
	defer s.Close()
	return binary.Write(s.Conn, binary.LittleEndian, false)
}

//get task info
func (s *Conn) GetHostInfo() (h *file.Host, err error) {
	var l int
	var b []byte
	if l, err = s.GetLen(); err != nil {
		return
	} else if b, err = s.ReadLen(l); err != nil {
		return
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		h = new(file.Host)
		h.Host = arr[0]
		h.Target = arr[1]
		h.HeaderChange = arr[2]
		h.HostChange = arr[3]
		h.Remark = arr[4]
		h.Location = arr[5]
		h.Flow = new(file.Flow)
		h.NoStore = true
	}
	return
}

//send task info
func (s *Conn) SendConfigInfo(c *file.Config) (int, error) {
	/*
		The task info is formed as follows:
		+----+-----+---------+
		|type| len | content |
		+----+---------------+
		| 4  |  4  |   ...   |
		+----+---------------+
	*/
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte(common.NEW_CONF))
	common.BinaryWrite(raw, c.U, c.P, common.GetStrByBool(c.Crypt), c.Compress)
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
}

//get task info
func (s *Conn) GetConfigInfo() (c *file.Config, err error) {
	var l int
	var b []byte
	if l, err = s.GetLen(); err != nil {
		return
	} else if b, err = s.ReadLen(l); err != nil {
		return
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		c = new(file.Config)
		c.U = arr[0]
		c.P = arr[1]
		c.Crypt = common.GetBoolByStr(arr[2])
		c.Compress = arr[3]
		c.CompressDecode, c.CompressDecode = common.GetCompressType(arr[3])
	}
	return
}

//send task info
func (s *Conn) SendTaskInfo(t *file.Tunnel) (int, error) {
	/*
		The task info is formed as follows:
		+----+-----+---------+
		|type| len | content |
		+----+---------------+
		| 4  |  4  |   ...   |
		+----+---------------+
	*/
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte(common.NEW_TASK))
	common.BinaryWrite(raw, t.Mode, t.Ports, t.Target, t.Remark)
	s.Lock()
	defer s.Unlock()
	return s.Write(raw.Bytes())
}

//get task info
func (s *Conn) GetTaskInfo() (t *file.Tunnel, err error) {
	var l int
	var b []byte
	if l, err = s.GetLen(); err != nil {
		return
	} else if b, err = s.ReadLen(l); err != nil {
		return
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		t = new(file.Tunnel)
		t.Mode = arr[0]
		t.Ports = arr[1]
		t.Target = arr[2]
		t.Id = file.GetCsvDb().GetTaskId()
		t.Status = true
		t.Flow = new(file.Flow)
		t.Remark = arr[3]
		t.NoStore = true
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
	return s.Write([]byte(common.RES_MSG))
}

//write sign flag
func (s *Conn) WriteSign() (int, error) {
	return s.Write([]byte(common.RES_SIGN))
}

//write sign flag
func (s *Conn) WriteClose() (int, error) {
	return s.Write([]byte(common.RES_CLOSE))
}

//write main
func (s *Conn) WriteMain() (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.Write([]byte(common.WORK_MAIN))
}

//write main
func (s *Conn) WriteConfig() (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.Write([]byte(common.WORK_CONFIG))
}

//write chan
func (s *Conn) WriteChan() (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.Write([]byte(common.WORK_CHAN))
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

func SetUdpSession(sess *kcp.UDPSession) {
	sess.SetStreamMode(true)
	sess.SetWindowSize(1024, 1024)
	sess.SetReadBuffer(64 * 1024)
	sess.SetWriteBuffer(64 * 1024)
	sess.SetNoDelay(1, 10, 2, 1)
	sess.SetMtu(1600)
	sess.SetACKNoDelay(true)
}
