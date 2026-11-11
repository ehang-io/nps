package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/ftp"
	"github.com/astaxie/beego"
)

// directBridge 把隧道的「内网一侧」直接换成一次本地拨号：
// 测试里没有 npc，SendLinkInfo 就是「让内网那台机器去连这个地址」的替身。
type directBridge struct{}

func (directBridge) SendLinkInfo(clientId int, link *conn.Link, t *file.Tunnel) (net.Conn, error) {
	return net.DialTimeout("tcp", link.Host, 3*time.Second)
}

// fakeFtpServer 只实现测试需要的 FTP 命令。
// 被动模式的数据端口是随机临时端口、并且按内网地址通告，
// 这正是穿透场景下公网客户端连不上的根因。
type fakeFtpServer struct {
	ln      net.Listener
	payload string

	mu       sync.Mutex
	active   string
	dataLn   net.Listener
	pasvPort int
}

func newFakeFtpServer(t *testing.T, payload string) *fakeFtpServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ftp: %v", err)
	}
	f := &fakeFtpServer{ln: ln, payload: payload}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeFtpServer) Addr() string { return f.ln.Addr().String() }

func (f *fakeFtpServer) Close() {
	_ = f.ln.Close()
	f.mu.Lock()
	if f.dataLn != nil {
		_ = f.dataLn.Close()
	}
	f.mu.Unlock()
}

// lastPasvPort 返回最近一次 PASV/EPSV 通告的内网数据端口
func (f *fakeFtpServer) lastPasvPort() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pasvPort
}

func (f *fakeFtpServer) serve(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	w := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
	w("220 fake ftp ready")
	br := bufio.NewReader(c)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		verb, rest := splitCmd(line)
		switch verb {
		case "USER":
			w("331 need password")
		case "PASS":
			w("230 logged in")
		case "TYPE", "OPTS", "SYST":
			w("200 ok")
		case "PASV":
			port := f.openPassive()
			f.mu.Lock()
			f.pasvPort = port
			f.mu.Unlock()
			// 通告的是内网地址 + 随机端口，公网客户端不可能直接连上
			w(fmt.Sprintf("227 Entering Passive Mode (127,0,0,1,%d,%d)", port>>8, port&0xff))
		case "EPSV":
			port := f.openPassive()
			f.mu.Lock()
			f.pasvPort = port
			f.mu.Unlock()
			w(fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", port))
		case "PORT":
			addr, ok := ftp.ParsePort(line)
			if !ok {
				w("501 bad port")
				continue
			}
			f.mu.Lock()
			f.active = addr
			f.mu.Unlock()
			w("200 port ok")
		case "EPRT":
			addr, ok := ftp.ParseEprt(line)
			if !ok {
				w("501 bad eprt")
				continue
			}
			f.mu.Lock()
			f.active = addr
			f.mu.Unlock()
			w("200 eprt ok")
		case "RETR", "LIST":
			w("150 opening data connection")
			if err := f.sendData(); err != nil {
				w("425 can not open data connection")
				continue
			}
			w("226 transfer complete")
		case "QUIT":
			w("221 bye")
			return
		default:
			_ = rest
			w("200 ok")
		}
	}
}

func (f *fakeFtpServer) openPassive() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dataLn != nil {
		_ = f.dataLn.Close()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	f.dataLn = ln
	return ln.Addr().(*net.TCPAddr).Port
}

// sendData 主动模式拨回客户端通告的地址，被动模式等客户端连上来
func (f *fakeFtpServer) sendData() error {
	f.mu.Lock()
	active := f.active
	ln := f.dataLn
	f.active = ""
	f.mu.Unlock()
	var dc net.Conn
	var err error
	if active != "" {
		dc, err = net.DialTimeout("tcp", active, 5*time.Second)
	} else if ln != nil {
		if tl, ok := ln.(*net.TCPListener); ok {
			_ = tl.SetDeadline(time.Now().Add(5 * time.Second))
		}
		dc, err = ln.Accept()
	} else {
		return fmt.Errorf("no data channel")
	}
	if err != nil {
		return err
	}
	defer dc.Close()
	_, err = dc.Write([]byte(f.payload))
	return err
}

func splitCmd(line string) (string, string) {
	s := strings.TrimRight(line, "\r\n")
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return strings.ToUpper(s[:i]), strings.TrimSpace(s[i+1:])
	}
	return strings.ToUpper(s), ""
}

// newFtpTestProxy 起一个只会跑 ProcessFtp 的最小服务端，返回它监听的公网端口
func newFtpTestProxy(t *testing.T, targetAddr string, mode string) string {
	t.Helper()
	client := file.NewClient("vkey", true, true)
	client.Id = 1
	client.Flow = new(file.Flow)
	task := &file.Tunnel{
		Id:     1,
		Port:   21021,
		Mode:   mode,
		Client: client,
		Flow:   new(file.Flow),
		Target: &file.Target{TargetStr: targetAddr},
	}
	s := NewTunnelModeServer(ProcessFtp, directBridge{}, task)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen public: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = ProcessFtp(conn.NewConn(c), s)
			}()
		}
	}()
	return ln.Addr().String()
}

// readResponse 读一条完整的 FTP 响应（多行响应读到 "123 " 结束）
func readResponse(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if len(line) >= 4 && (line[3] == ' ' || line[3] == '\r') {
			return strings.TrimRight(line, "\r\n")
		}
	}
}

func sendCmd(t *testing.T, c net.Conn, cmd string) {
	t.Helper()
	if _, err := c.Write([]byte(cmd + "\r\n")); err != nil {
		t.Fatalf("send %q: %v", cmd, err)
	}
}

// 被动模式：227 里的内网地址和随机端口必须被改写成公网地址 + 中继端口，
// 客户端连中继端口后数据要经隧道回到内网 FTP 的数据端口。
func TestProcessFtpPassiveMode(t *testing.T) {
	if err := beego.AppConfig.Set("ftp_public_ip", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := beego.AppConfig.Set("ftp_pasv_ports", ""); err != nil {
		t.Fatal(err)
	}
	ftpSrv := newFakeFtpServer(t, "PASSIVE_OK")
	defer ftpSrv.Close()
	publicAddr := newFtpTestProxy(t, ftpSrv.Addr(), "ftp")

	c, err := net.DialTimeout("tcp", publicAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	br := bufio.NewReader(c)
	if first := readResponse(t, br); !strings.HasPrefix(first, "220") {
		t.Fatalf("unexpected greeting %q", first)
	}
	sendCmd(t, c, "USER u")
	if got := readResponse(t, br); !strings.HasPrefix(got, "331") {
		t.Fatalf("USER: %q", got)
	}
	sendCmd(t, c, "PASS p")
	if got := readResponse(t, br); !strings.HasPrefix(got, "230") {
		t.Fatalf("PASS: %q", got)
	}
	sendCmd(t, c, "PASV")
	pasvLine := readResponse(t, br)
	host, port, ok := ftp.ParsePasv(pasvLine)
	if !ok {
		t.Fatalf("PASV response not parsable: %q", pasvLine)
	}
	if host != "127.0.0.1" {
		t.Fatalf("passive address not rewritten, got %q", host)
	}
	if port == ftpSrv.lastPasvPort() {
		t.Fatalf("passive port %d was not relayed (internal port leaked)", port)
	}

	dc, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		t.Fatalf("dial relayed passive port: %v", err)
	}
	defer dc.Close()
	sendCmd(t, c, "RETR file")
	if got := readResponse(t, br); !strings.HasPrefix(got, "150") {
		t.Fatalf("RETR: %q", got)
	}
	payload, err := io.ReadAll(io.LimitReader(dc, 64))
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if string(payload) != "PASSIVE_OK" {
		t.Fatalf("passive payload = %q", payload)
	}
}

// 主动模式：PORT 里客户端自己的地址要被换成服务端中继端口，
// FTP 服务连到中继端口后，数据要能回到客户端通告的地址。
func TestProcessFtpActiveMode(t *testing.T) {
	if err := beego.AppConfig.Set("ftp_public_ip", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := beego.AppConfig.Set("ftp_active_ports", ""); err != nil {
		t.Fatal(err)
	}
	ftpSrv := newFakeFtpServer(t, "ACTIVE_OK")
	defer ftpSrv.Close()
	publicAddr := newFtpTestProxy(t, ftpSrv.Addr(), "ftp")

	// 客户端自己的数据端口（真实场景在客户端机器上，靠 nps 中继回来）
	dataLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen client data: %v", err)
	}
	defer dataLn.Close()
	dataPort := dataLn.Addr().(*net.TCPAddr).Port

	c, err := net.DialTimeout("tcp", publicAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	br := bufio.NewReader(c)
	readResponse(t, br) // 220
	sendCmd(t, c, fmt.Sprintf("PORT 127,0,0,1,%d,%d", dataPort>>8, dataPort&0xff))
	if got := readResponse(t, br); !strings.HasPrefix(got, "200") {
		t.Fatalf("PORT: %q", got)
	}

	sendCmd(t, c, "RETR file")
	if got := readResponse(t, br); !strings.HasPrefix(got, "150") {
		t.Fatalf("RETR: %q", got)
	}
	if tl, ok := dataLn.(*net.TCPListener); ok {
		_ = tl.SetDeadline(time.Now().Add(10 * time.Second))
	}
	dc, err := dataLn.Accept()
	if err != nil {
		t.Fatalf("client data port was not relayed: %v", err)
	}
	defer dc.Close()
	payload, err := io.ReadAll(io.LimitReader(dc, 64))
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if string(payload) != "ACTIVE_OK" {
		t.Fatalf("active payload = %q", payload)
	}
}

// AUTH TLS 之后控制通道是加密的，改写必须停掉，
// 至少要保证数据通道编号原样透传而不是被改坏。
func TestProcessFtpDisablesRewriteAfterAuthTls(t *testing.T) {
	if err := beego.AppConfig.Set("ftp_public_ip", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	ftpSrv := newFakeFtpServer(t, "x")
	defer ftpSrv.Close()
	publicAddr := newFtpTestProxy(t, ftpSrv.Addr(), "ftp")

	c, err := net.DialTimeout("tcp", publicAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	br := bufio.NewReader(c)
	readResponse(t, br) // 220
	sendCmd(t, c, "AUTH TLS")
	readResponse(t, br) // 200
	sendCmd(t, c, "PASV")
	line := readResponse(t, br)
	_, port, ok := ftp.ParsePasv(line)
	if !ok {
		t.Fatalf("PASV response not parsable: %q", line)
	}
	if port != ftpSrv.lastPasvPort() {
		t.Fatalf("rewrite should be disabled after AUTH TLS, got port %d want %d", port, ftpSrv.lastPasvPort())
	}
}

// FtpAlgForTunnel 决定老 tcp 隧道要不要自动套上 FTP 改写
func TestFtpAlgForTunnel(t *testing.T) {
	if err := beego.AppConfig.Set("ftp_alg", "auto"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		task *file.Tunnel
		want bool
	}{
		{"mode ftp", &file.Tunnel{Mode: "ftp"}, true},
		{"tcp on public 21", &file.Tunnel{Mode: "tcp", Port: 21}, true},
		{"tcp targeting 21", &file.Tunnel{Mode: "tcp", Port: 21021, Target: &file.Target{TargetStr: "10.0.0.8:21"}}, true},
		{"tcp unrelated", &file.Tunnel{Mode: "tcp", Port: 3389, Target: &file.Target{TargetStr: "10.0.0.8:3389"}}, false},
		{"udp", &file.Tunnel{Mode: "udp", Port: 21}, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := FtpAlgForTunnel(tc.task); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
	if err := beego.AppConfig.Set("ftp_alg", "off"); err != nil {
		t.Fatal(err)
	}
	if FtpAlgForTunnel(&file.Tunnel{Mode: "tcp", Port: 21}) {
		t.Fatal("ftp_alg=off must disable the automatic rewrite")
	}
	_ = beego.AppConfig.Set("ftp_alg", "auto")
}

// 端口范围被占满时必须报错，而不是悄悄换成随机高位端口：
// 随机端口几乎不可能在云安全组放行，客户端只会连上去然后一直卡在 150
func TestListenFtpRelayExhaustedRange(t *testing.T) {
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen busy port: %v", err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	if ln, err := listenFtpRelay(strconv.Itoa(port)); err == nil {
		_ = ln.Close()
		t.Fatalf("expected error when the only relay port %d is taken, got listener", port)
	}
	// 范围里还有空位时要用配置的端口，而不是随机端口
	free, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen temp: %v", err)
	}
	freePort := free.Addr().(*net.TCPAddr).Port
	_ = free.Close()
	ln, err := listenFtpRelay(strconv.Itoa(freePort))
	if err != nil {
		t.Fatalf("listen relay on %d: %v", freePort, err)
	}
	defer ln.Close()
	if got := ln.Addr().(*net.TCPAddr).Port; got != freePort {
		t.Fatalf("relay should use configured port %d, got %d", freePort, got)
	}
}

// PASV/PORT/EPRT 必须在报文里带地址，没有公网 IPv4 时只能放弃；
// EPSV 只带端口，纯 IPv6 或多层 NAT 环境下照样能用
func TestFtpRelayNeedsPublicIPOnlyForAddressModes(t *testing.T) {
	task := &file.Tunnel{Id: 1, Port: 21021}
	s := NewTunnelModeServer(ProcessFtp, directBridge{}, task)
	sess := newFtpSession(s, nil, nil, "10.0.0.8:21")
	sess.publicIP = ""

	if _, err := sess.openRelay("", true, false, func() string { return "" }); err == nil {
		t.Fatal("relay that advertises an address must fail without a public ip")
	}
	port, err := sess.openRelay("", false, false, func() string { return "" })
	if err != nil || port <= 0 {
		t.Fatalf("EPSV relay must work without a public ip, port=%d err=%v", port, err)
	}
	sess.shutdown()
}

// 中继端口是公网可达的裸端口，必须跟控制通道用同一套来源策略，
// 否则扫描器可以绕过白名单直接连数据端口
func TestFtpDataChannelHonoursSourceGuard(t *testing.T) {
	task := &file.Tunnel{Id: 1, Port: 21021}
	s := NewTunnelModeServer(ProcessFtp, directBridge{}, task)
	s.guard = common.NewAccessGuard("127.0.0.1", "203.0.113.9", 0, time.Second, time.Minute)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	dc, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer dc.Close()

	sess := &ftpSession{srv: s, peerIP: "127.0.0.1"}
	if !sess.dataSourceAllowed(dc, false) {
		t.Fatal("passive data channel from the control source must pass")
	}
	if !sess.dataSourceAllowed(dc, true) {
		t.Fatal("active data channel is opened by the intranet server, allow-list must not block it")
	}
	// 控制连接来源变了以后，来自 127.0.0.1 的数据连接不再被视为同源，
	// 命中白名单以外的地址就要拒绝
	sess.peerIP = "198.51.100.7"
	if !sess.dataSourceAllowed(dc, false) {
		t.Fatal("127.0.0.1 is still inside the allow list, must pass")
	}
	// 白名单里没有 127.0.0.1 时，非同源的数据连接必须拒绝
	s.guard = common.NewAccessGuard("198.51.100.7", "", 0, time.Second, time.Minute)
	if sess.dataSourceAllowed(dc, false) {
		t.Fatal("data channel from an address outside the allow list must be rejected")
	}
	// 主动模式由内网 FTP 服务发起，只判黑名单，不套白名单
	if !sess.dataSourceAllowed(dc, true) {
		t.Fatal("active data channel must not be blocked by the allow list")
	}
	s.guard = common.NewAccessGuard("", "127.0.0.1", 0, time.Second, time.Minute)
	if sess.dataSourceAllowed(dc, true) {
		t.Fatal("active data channel from a denied address must be rejected")
	}
}
