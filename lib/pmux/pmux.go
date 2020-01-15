// This module is used for port reuse
// Distinguish client, web manager , HTTP and HTTPS according to the difference of protocol
package pmux

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"github.com/astaxie/beego/logs"
	"github.com/pkg/errors"
)

const (
	HTTP_GET        = 716984
	HTTP_POST       = 807983
	HTTP_HEAD       = 726965
	HTTP_PUT        = 808585
	HTTP_DELETE     = 686976
	HTTP_CONNECT    = 677978
	HTTP_OPTIONS    = 798084
	HTTP_TRACE      = 848265
	CLIENT          = 848384
	ACCEPT_TIME_OUT = 10
)

type PortMux struct {
	net.Listener
	port        int
	isClose     bool
	managerHost string
	clientConn  chan *PortConn
	httpConn    chan *PortConn
	httpsConn   chan *PortConn
	managerConn chan *PortConn
}

func NewPortMux(port int, managerHost string) *PortMux {
	pMux := &PortMux{
		managerHost: managerHost,
		port:        port,
		clientConn:  make(chan *PortConn),
		httpConn:    make(chan *PortConn),
		httpsConn:   make(chan *PortConn),
		managerConn: make(chan *PortConn),
	}
	pMux.Start()
	return pMux
}

func (pMux *PortMux) Start() error {
	// Port multiplexing is based on TCP only
	tcpAddr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:"+strconv.Itoa(pMux.port))
	if err != nil {
		return err
	}
	pMux.Listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logs.Error(err)
		os.Exit(0)
	}
	go func() {
		for {
			conn, err := pMux.Listener.Accept()
			if err != nil {
				logs.Warn(err)
				//close
				pMux.Close()
			}
			go pMux.process(conn)
		}
	}()
	return nil
}

func (pMux *PortMux) process(conn net.Conn) {
	// Recognition according to different signs
	// read 3 byte
	buf := make([]byte, 3)
	if n, err := io.ReadFull(conn, buf); err != nil || n != 3 {
		return
	}
	var ch chan *PortConn
	var rs []byte
	var buffer bytes.Buffer
	var readMore = false
	switch common.BytesToNum(buf) {
	case HTTP_CONNECT, HTTP_DELETE, HTTP_GET, HTTP_HEAD, HTTP_OPTIONS, HTTP_POST, HTTP_PUT, HTTP_TRACE: //http and manager
		buffer.Reset()
		r := bufio.NewReader(conn)
		buffer.Write(buf)
		for {
			b, _, err := r.ReadLine()
			if err != nil {
				logs.Warn("read line error", err.Error())
				conn.Close()
				break
			}
			buffer.Write(b)
			buffer.Write([]byte("\r\n"))
			if strings.Index(string(b), "Host:") == 0 || strings.Index(string(b), "host:") == 0 {
				// Remove host and space effects
				str := strings.Replace(string(b), "Host:", "", -1)
				str = strings.Replace(str, "host:", "", -1)
				str = strings.TrimSpace(str)
				// Determine whether it is the same as the manager domain name
				if common.GetIpByAddr(str) == pMux.managerHost {
					ch = pMux.managerConn
				} else {
					ch = pMux.httpConn
				}
				b, _ := r.Peek(r.Buffered())
				buffer.Write(b)
				rs = buffer.Bytes()
				break
			}
		}
	case CLIENT: // client connection
		ch = pMux.clientConn
	default: // https
		readMore = true
		ch = pMux.httpsConn
	}
	if len(rs) == 0 {
		rs = buf
	}
	timer := time.NewTimer(ACCEPT_TIME_OUT)
	select {
	case <-timer.C:
	case ch <- newPortConn(conn, rs, readMore):
	}
}

func (pMux *PortMux) Close() error {
	if pMux.isClose {
		return errors.New("the port pmux has closed")
	}
	pMux.isClose = true
	close(pMux.clientConn)
	close(pMux.httpsConn)
	close(pMux.httpConn)
	close(pMux.managerConn)
	return pMux.Listener.Close()
}

func (pMux *PortMux) GetClientListener() net.Listener {
	return NewPortListener(pMux.clientConn, pMux.Listener.Addr())
}

func (pMux *PortMux) GetHttpListener() net.Listener {
	return NewPortListener(pMux.httpConn, pMux.Listener.Addr())
}

func (pMux *PortMux) GetHttpsListener() net.Listener {
	return NewPortListener(pMux.httpsConn, pMux.Listener.Addr())
}

func (pMux *PortMux) GetManagerListener() net.Listener {
	return NewPortListener(pMux.managerConn, pMux.Listener.Addr())
}
