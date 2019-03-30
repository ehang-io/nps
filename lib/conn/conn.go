package conn

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/mux"
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

type Conn struct {
	Conn net.Conn
	Rb   []byte
}

//new conn
func NewConn(conn net.Conn) *Conn {
	return &Conn{Conn: conn}
}

//get host 、connection type、method...from connection
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
	if hostPortURL.Opaque == "443" {
		if strings.Index(r.Host, ":") == -1 {
			address = r.Host + ":443"
		} else {
			address = r.Host
		}
	} else {
		if strings.Index(r.Host, ":") == -1 {
			address = r.Host + ":80"
		} else {
			address = r.Host
		}
	}
	return
}

func (s *Conn) GetShortLenContent() (b []byte, err error) {
	var l int
	if l, err = s.GetLen(); err != nil {
		return
	}
	if l < 0 || l > 32<<10 {
		err = errors.New("read length error")
		return
	}
	return s.GetShortContent(l)
}

func (s *Conn) GetShortContent(l int) (b []byte, err error) {
	buf := make([]byte, l)
	return buf, binary.Read(s, binary.LittleEndian, &buf)
}

func (s *Conn) LocalAddr() net.Addr {
	return s.Conn.LocalAddr()
}

func (s *Conn) RemoteAddr() net.Addr {
	return s.Conn.RemoteAddr()
}

func (s *Conn) SetDeadline(t time.Time) error {
	return s.Conn.SetDeadline(t)
}

func (s *Conn) SetWriteDeadline(t time.Time) error {
	return s.Conn.SetWriteDeadline(t)
}

func (s *Conn) SetReadDeadline(t time.Time) error {
	return s.Conn.SetReadDeadline(t)
}

//读取指定长度内容
func (s *Conn) ReadLen(cLen int, buf []byte) (int, error) {
	if cLen > len(buf) {
		return 0, errors.New("长度错误" + strconv.Itoa(cLen))
	}
	if n, err := io.ReadFull(s, buf[:cLen]); err != nil || n != cLen {
		return n, errors.New("Error reading specified length " + err.Error())
	}
	return cLen, nil
}

func (s *Conn) GetLen() (int, error) {
	var l int32
	err := binary.Read(s, binary.LittleEndian, &l)
	return int(l), err
}

func (s *Conn) WriteLenContent(buf []byte) (err error) {
	var b []byte
	if b, err = GetLenBytes(buf); err != nil {
		return
	}
	return binary.Write(s.Conn, binary.LittleEndian, b)
}

//read flag
func (s *Conn) ReadFlag() (string, error) {
	buf := make([]byte, 4)
	return string(buf), binary.Read(s, binary.LittleEndian, &buf)
}

//set alive
func (s *Conn) SetAlive(tp string) {
	switch s.Conn.(type) {
	case *kcp.UDPSession:
		s.Conn.(*kcp.UDPSession).SetReadDeadline(time.Time{})
	case *net.TCPConn:
		conn := s.Conn.(*net.TCPConn)
		conn.SetReadDeadline(time.Time{})
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(time.Duration(2 * time.Second))
	case *mux.PortConn:
		s.Conn.(*mux.PortConn).SetReadDeadline(time.Time{})
	}
}

//set read deadline
func (s *Conn) SetReadDeadlineByType(t time.Duration, tp string) {
	switch s.Conn.(type) {
	case *kcp.UDPSession:
		s.Conn.(*kcp.UDPSession).SetReadDeadline(time.Now().Add(time.Duration(t) * time.Second))
	case *net.TCPConn:
		s.Conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(t) * time.Second))
	case *mux.PortConn:
		s.Conn.(*mux.PortConn).SetReadDeadline(time.Now().Add(time.Duration(t) * time.Second))
	}
}

//send info for link
func (s *Conn) SendLinkInfo(link *Link) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	common.BinaryWrite(raw, link.ConnType, link.Host, common.GetStrByBool(link.Compress), common.GetStrByBool(link.Crypt), link.RemoteAddr)
	return s.Write(raw.Bytes())
}

//get link info from conn
func (s *Conn) GetLinkInfo() (lk *Link, err error) {
	lk = new(Link)
	var l int
	buf := pool.BufPoolMax.Get().([]byte)
	defer pool.PutBufPoolMax(buf)
	if l, err = s.GetLen(); err != nil {
		return
	} else if _, err = s.ReadLen(l, buf); err != nil {
		return
	} else {
		arr := strings.Split(string(buf[:l]), common.CONN_DATA_SEQ)
		lk.ConnType = arr[0]
		lk.Host = arr[1]
		lk.Compress = common.GetBoolByStr(arr[2])
		lk.Crypt = common.GetBoolByStr(arr[3])
		lk.RemoteAddr = arr[4]
	}
	return
}

//send info for link
func (s *Conn) SendHealthInfo(info, status string) (int, error) {
	raw := bytes.NewBuffer([]byte{})
	common.BinaryWrite(raw, info, status)
	return s.Write(raw.Bytes())
}

//get health info from conn
func (s *Conn) GetHealthInfo() (info string, status bool, err error) {
	var l int
	buf := pool.BufPoolMax.Get().([]byte)
	defer pool.PutBufPoolMax(buf)
	if l, err = s.GetLen(); err != nil {
		return
	} else if _, err = s.ReadLen(l, buf); err != nil {
		return
	} else {
		arr := strings.Split(string(buf[:l]), common.CONN_DATA_SEQ)
		if len(arr) >= 2 {
			return arr[0], common.GetBoolByStr(arr[1]), nil
		}
	}
	return "", false, errors.New("receive health info error")
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
	common.BinaryWrite(raw, h.Host, h.Target.TargetStr, h.HeaderChange, h.HostChange, h.Remark, h.Location, h.Scheme)
	return s.Write(raw.Bytes())
}

//get task info
func (s *Conn) GetHostInfo() (h *file.Host, err error) {
	var l int
	buf := pool.BufPoolMax.Get().([]byte)
	defer pool.PutBufPoolMax(buf)
	if l, err = s.GetLen(); err != nil {
		return
	} else if _, err = s.ReadLen(l, buf); err != nil {
		return
	} else {
		arr := strings.Split(string(buf[:l]), common.CONN_DATA_SEQ)
		h = new(file.Host)
		h.Target = new(file.Target)
		h.Id = int(file.GetDb().JsonDb.GetHostId())
		h.Host = arr[0]
		h.Target.TargetStr = arr[1]
		h.HeaderChange = arr[2]
		h.HostChange = arr[3]
		h.Remark = arr[4]
		h.Location = arr[5]
		h.Scheme = arr[6]
		if h.Scheme == "" {
			h.Scheme = "all"
		}
		h.Flow = new(file.Flow)
		h.NoStore = true
	}
	return
}

//send task info
func (s *Conn) SendConfigInfo(c *config.CommonConfig) (int, error) {
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
	common.BinaryWrite(raw, c.Cnf.U, c.Cnf.P, common.GetStrByBool(c.Cnf.Crypt), common.GetStrByBool(c.Cnf.Compress), strconv.Itoa(c.Client.RateLimit),
		strconv.Itoa(int(c.Client.Flow.FlowLimit)), strconv.Itoa(c.Client.MaxConn), c.Client.Remark)
	return s.Write(raw.Bytes())
}

//get task info
func (s *Conn) GetConfigInfo() (c *file.Client, err error) {
	var l int
	buf := pool.BufPoolMax.Get().([]byte)
	defer pool.PutBufPoolMax(buf)
	if l, err = s.GetLen(); err != nil {
		return
	} else if _, err = s.ReadLen(l, buf); err != nil {
		return
	} else {
		arr := strings.Split(string(buf[:l]), common.CONN_DATA_SEQ)
		c = file.NewClient("", true, false)
		c.Cnf.U = arr[0]
		c.Cnf.P = arr[1]
		c.Cnf.Crypt = common.GetBoolByStr(arr[2])
		c.Cnf.Compress = common.GetBoolByStr(arr[3])
		c.RateLimit = common.GetIntNoErrByStr(arr[4])
		c.Flow.FlowLimit = int64(common.GetIntNoErrByStr(arr[5]))
		c.MaxConn = common.GetIntNoErrByStr(arr[6])
		c.Remark = arr[7]
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
	common.BinaryWrite(raw, t.Mode, t.Ports, t.Target.TargetStr, t.Remark, t.TargetAddr, t.Password, t.LocalPath, t.StripPre, t.ServerIp)
	return s.Write(raw.Bytes())
}

//get task info
func (s *Conn) GetTaskInfo() (t *file.Tunnel, err error) {
	var l int
	buf := pool.BufPoolMax.Get().([]byte)
	defer pool.PutBufPoolMax(buf)
	if l, err = s.GetLen(); err != nil {
		return
	} else if _, err = s.ReadLen(l, buf); err != nil {
		return
	} else {
		arr := strings.Split(string(buf[:l]), common.CONN_DATA_SEQ)
		t = new(file.Tunnel)
		t.Target = new(file.Target)
		t.Mode = arr[0]
		t.Ports = arr[1]
		t.Target.TargetStr = arr[2]
		t.Id = int(file.GetDb().JsonDb.GetTaskId())
		t.Status = true
		t.Flow = new(file.Flow)
		t.Remark = arr[3]
		t.TargetAddr = arr[4]
		t.Password = arr[5]
		t.LocalPath = arr[6]
		t.StripPre = arr[7]
		if len(arr) > 8 {
			t.ServerIp = arr[8]
		}
		t.NoStore = true
	}
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
func (s *Conn) Read(b []byte) (n int, err error) {
	if s.Rb != nil {
		if len(s.Rb) > 0 {
			n = copy(b, s.Rb)
			s.Rb = s.Rb[n:]
			return
		}
		s.Rb = nil
	}
	return s.Conn.Read(b)
}

//write sign flag
func (s *Conn) WriteClose() (int, error) {
	return s.Write([]byte(common.RES_CLOSE))
}

//write main
func (s *Conn) WriteMain() (int, error) {
	return s.Write([]byte(common.WORK_MAIN))
}

//write main
func (s *Conn) WriteConfig() (int, error) {
	return s.Write([]byte(common.WORK_CONFIG))
}

//write chan
func (s *Conn) WriteChan() (int, error) {
	return s.Write([]byte(common.WORK_CHAN))
}

//get task or host result of add
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

//get the assembled amount data(len 4 and content)
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

//udp connection setting
func SetUdpSession(sess *kcp.UDPSession) {
	sess.SetStreamMode(true)
	sess.SetWindowSize(1024, 1024)
	sess.SetReadBuffer(64 * 1024)
	sess.SetWriteBuffer(64 * 1024)
	sess.SetNoDelay(1, 10, 2, 1)
	sess.SetMtu(1600)
	sess.SetACKNoDelay(true)
	sess.SetWriteDelay(false)
}

//conn1 mux conn
func CopyWaitGroup(conn1, conn2 net.Conn, crypt bool, snappy bool, rate *rate.Rate, flow *file.Flow, isServer bool, rb []byte) {
	var in, out int64
	var wg sync.WaitGroup
	connHandle := GetConn(conn1, crypt, snappy, rate, isServer)
	if rb != nil {
		connHandle.Write(rb)
	}
	go func(in *int64) {
		wg.Add(1)
		*in, _ = common.CopyBuffer(connHandle, conn2)
		connHandle.Close()
		conn2.Close()
		wg.Done()
	}(&in)
	out, _ = common.CopyBuffer(conn2, connHandle)
	connHandle.Close()
	conn2.Close()
	wg.Wait()
	if flow != nil {
		flow.Add(in, out)
	}
}

//get crypt or snappy conn
func GetConn(conn net.Conn, cpt, snappy bool, rt *rate.Rate, isServer bool) (io.ReadWriteCloser) {
	if cpt {
		if isServer {
			return rate.NewRateConn(crypt.NewTlsServerConn(conn), rt)
		}
		return rate.NewRateConn(crypt.NewTlsClientConn(conn), rt)
	} else if snappy {
		return rate.NewRateConn(NewSnappyConn(conn), rt)
	}
	return rate.NewRateConn(conn, rt)
}
