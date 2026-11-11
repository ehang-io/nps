package proxy

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/ftp"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

// FTP 数据通道中继的默认参数
const (
	// ftpRelayDefaultIdle 单个中继端口等待数据连接的时间：
	// PASV 之后客户端通常在一秒内就连上来，60 秒足够，且不会让端口长期被占
	ftpRelayDefaultIdle = 60 * time.Second
	// ftpMaxRelayPerSess 单条控制连接同时保留的中继端口上限，
	// 超过时关掉最早的那个（FTP 客户端每次传输都会重新协商端口）
	ftpMaxRelayPerSess = 8
	// ftpDataDialTimeout 数据通道建立隧道的超时
	ftpDataDialTimeout = 10 * time.Second
)

// FtpAlgForTunnel 返回该隧道是否需要改写 FTP 数据通道。
//
// 判定规则：
//  1. mode=ftp 的隧道一律启用
//  2. mode=tcp 的隧道，如果 nps.conf 的 ftp_alg 是 auto（默认），
//     且公网端口或内网目标端口是 21，则自动启用（老用户不用改任何配置）
//
// 主动/被动模式的数据端口是随机的，不改写就只能靠 FTP 客户端的“忽略 PASV 地址”
// 之类的私有开关，绝大多数客户端没有这个选项。
func FtpAlgForTunnel(task *file.Tunnel) bool {
	if task == nil {
		return false
	}
	if task.Mode == "ftp" {
		return true
	}
	if task.Mode != "tcp" && task.Mode != "file" {
		return false
	}
	if beego.AppConfig.DefaultString("ftp_alg", "auto") != "auto" {
		return false
	}
	return task.Port == 21 || targetPort(task) == 21
}

// ProcessFtp 转发 FTP 控制通道，并改写两个方向上的数据通道信息：
//
//	客户端 -> 服务端：PORT/EPRT 里的地址换成 nps 的中继端口，服务端连过来后
//	                  再把这条数据连接交给客户端通告的地址
//	服务端 -> 客户端：227/229 里的内网地址+随机端口换成 nps 的公网地址+中继端口，
//	                  公网客户端连到中继端口后，经隧道转到内网 FTP 的数据端口
//
// 遇到 AUTH TLS 或者非文本流量时整条会话回退为直通（记录一次告警），
// 保证 FTPS 之类改不了的场景至少行为和改造前一致。
func ProcessFtp(c *conn.Conn, s *TunnelModeServer) error {
	targetAddr, err := s.task.Target.GetRandomTarget()
	if err != nil {
		c.Close()
		logs.Warn("ftp port %d, client id %d, task id %d connect error %s", s.task.Port, s.task.Client.Id, s.task.Id, err.Error())
		return err
	}
	lk := conn.NewLink(common.CONN_TCP, targetAddr, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, c.Conn.RemoteAddr().String(), s.task.Target.LocalProxy)
	target, err := s.bridge.SendLinkInfo(s.task.Client.Id, lk, s.task)
	if err != nil {
		logs.Notice("ftp: connect to target %s error %s", lk.Host, err.Error())
		c.Close()
		return err
	}
	remote := conn.GetConn(target, lk.Crypt, lk.Compress, s.task.Client.Rate, true)
	logs.Trace("new ftp connection, port %d, client id %d, remote %s, target %s", s.task.Port, s.task.Client.Id, c.RemoteAddr(), targetAddr)
	sess := newFtpSession(s, c, remote, targetAddr)
	sess.run()
	return nil
}

// ftpSession 是一条 FTP 控制连接对应的改写会话
type ftpSession struct {
	srv        *TunnelModeServer
	control    *conn.Conn
	remote     io.ReadWriteCloser
	targetHost string
	publicIP   string

	mu       sync.Mutex
	relays   []net.Listener
	disabled bool
	closed   bool
	warned   bool

	closeOnce sync.Once
	wg        sync.WaitGroup
}

func newFtpSession(srv *TunnelModeServer, control *conn.Conn, remote io.ReadWriteCloser, targetAddr string) *ftpSession {
	host := targetAddr
	if h, _, err := net.SplitHostPort(targetAddr); err == nil {
		host = h
	}
	s := &ftpSession{
		srv:        srv,
		control:    control,
		remote:     remote,
		targetHost: strings.Trim(host, "[]"),
		publicIP:   ftpPublicIPv4(control),
	}
	if s.publicIP == "" {
		logs.Warn("ftp port %d: no usable public ip (set ftp_public_ip or external_ip in nps.conf),"+
			" passive/active data channel will not be rewritten", srv.task.Port)
	}
	return s
}

// ftpPublicIPv4 依次取 ftp_public_ip、external_ip、控制连接的本地地址。
// PASV 的地址格式只支持 IPv4，因此只接受 IPv4 值。
func ftpPublicIPv4(control net.Conn) string {
	candidates := []string{
		strings.TrimSpace(beego.AppConfig.String("ftp_public_ip")),
		strings.TrimSpace(common.GetExternalIp()),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if ip := net.ParseIP(c); ip != nil && ip.To4() != nil {
			return ip.To4().String()
		}
	}
	if control != nil {
		if host, _, err := net.SplitHostPort(control.LocalAddr().String()); err == nil {
			if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
				return ip.To4().String()
			}
		}
	}
	return ""
}

func (s *ftpSession) run() {
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.copyClientToServer()
		s.shutdown()
	}()
	go func() {
		defer s.wg.Done()
		s.copyServerToClient()
		s.shutdown()
	}()
	s.wg.Wait()
}

// shutdown 关闭控制通道、隧道连接和所有中继端口，可重入
func (s *ftpSession) shutdown() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		relays := s.relays
		s.relays = nil
		s.mu.Unlock()
		for _, ln := range relays {
			_ = ln.Close()
		}
		if s.remote != nil {
			_ = s.remote.Close()
		}
		if s.control != nil {
			_ = s.control.Close()
		}
	})
}

// algActive 报告当前会话是否还在改写状态
func (s *ftpSession) algActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.disabled
}

// disable 停止改写并记录一次原因（同一条连接只记一次，避免刷日志）
func (s *ftpSession) disable(reason string) {
	s.mu.Lock()
	if s.disabled {
		s.mu.Unlock()
		return
	}
	s.disabled = true
	warned := s.warned
	s.warned = true
	s.mu.Unlock()
	if !warned {
		logs.Info("ftp port %d: data channel rewriting disabled for this session (%s)", s.srv.task.Port, reason)
	}
}

func (s *ftpSession) copyClientToServer() {
	br := bufio.NewReaderSize(s.control, ftp.MaxLine)
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			out := line
			if s.algActive() {
				if nb, ok := s.rewriteCommand(line); ok {
					out = nb
				}
			}
			n, werr := s.remote.Write(out)
			s.srv.FlowAdd(int64(n), 0)
			if werr != nil {
				return
			}
		}
		if err == bufio.ErrBufferFull {
			s.disable("command line too long")
			s.passthrough(br, true)
			return
		}
		if err != nil {
			return
		}
	}
}

func (s *ftpSession) copyServerToClient() {
	br := bufio.NewReaderSize(s.remote, ftp.MaxLine)
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			out := line
			if s.algActive() {
				if nb, ok := s.rewriteResponse(line); ok {
					out = nb
				}
			}
			n, werr := s.control.Write(out)
			s.srv.FlowAdd(0, int64(n))
			if werr != nil {
				return
			}
		}
		if err == bufio.ErrBufferFull {
			s.disable("response line too long")
			s.passthrough(br, false)
			return
		}
		if err != nil {
			return
		}
	}
}

// passthrough 把剩余字节原样搬运，用于改写不可行（二进制/超长行）的情况
func (s *ftpSession) passthrough(br *bufio.Reader, up bool) {
	buf := make([]byte, 32*1024)
	for {
		n, err := br.Read(buf)
		if n > 0 {
			var w int
			var werr error
			if up {
				w, werr = s.remote.Write(buf[:n])
				s.srv.FlowAdd(int64(w), 0)
			} else {
				w, werr = s.control.Write(buf[:n])
				s.srv.FlowAdd(0, int64(w))
			}
			if werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// rewriteCommand 处理客户端发往服务端的命令
func (s *ftpSession) rewriteCommand(line []byte) ([]byte, bool) {
	str := string(line)
	if ftp.IsAuthTls(str) {
		// 之后控制通道是 TLS，明文改写不再可行
		s.disable("client used AUTH TLS")
		return line, false
	}
	switch ftp.Verb(str) {
	case "PORT":
		addr, ok := ftp.ParsePort(str)
		if !ok {
			return line, false
		}
		port, err := s.openRelay(ftpActivePortSpec(), func() string { return addr })
		if err != nil {
			logs.Warn("ftp port %d: relay for PORT failed: %s", s.srv.task.Port, err.Error())
			return line, false
		}
		if nl, ok := ftp.SetPort(str, s.publicIP, port); ok {
			logs.Trace("ftp active mode: %s -> %s:%d", addr, s.publicIP, port)
			return []byte(nl), true
		}
	case "EPRT":
		addr, ok := ftp.ParseEprt(str)
		if !ok {
			return line, false
		}
		port, err := s.openRelay(ftpActivePortSpec(), func() string { return addr })
		if err != nil {
			logs.Warn("ftp port %d: relay for EPRT failed: %s", s.srv.task.Port, err.Error())
			return line, false
		}
		if nl, ok := ftp.SetEprt(str, s.publicIP, port); ok {
			logs.Trace("ftp active mode: %s -> %s:%d", addr, s.publicIP, port)
			return []byte(nl), true
		}
	}
	return line, false
}

// rewriteResponse 处理服务端发往客户端的响应
func (s *ftpSession) rewriteResponse(line []byte) ([]byte, bool) {
	str := string(line)
	switch ftp.Verb(str) {
	case "227":
		_, port, ok := ftp.ParsePasv(str)
		if !ok {
			return line, false
		}
		relay, err := s.openRelay(ftpPasvPortSpec(), func() string { return s.passiveTarget(port) })
		if err != nil {
			logs.Warn("ftp port %d: relay for PASV failed: %s", s.srv.task.Port, err.Error())
			return line, false
		}
		if nl, ok := ftp.SetPasv(str, s.publicIP, relay); ok {
			logs.Trace("ftp passive mode: internal port %d -> %s:%d", port, s.publicIP, relay)
			return []byte(nl), true
		}
	case "229":
		port, ok := ftp.ParseEpsv(str)
		if !ok {
			return line, false
		}
		relay, err := s.openRelay(ftpPasvPortSpec(), func() string { return s.passiveTarget(port) })
		if err != nil {
			logs.Warn("ftp port %d: relay for EPSV failed: %s", s.srv.task.Port, err.Error())
			return line, false
		}
		if nl, ok := ftp.SetEpsv(str, relay); ok {
			logs.Trace("ftp passive mode: internal port %d -> relay port %d", port, relay)
			return []byte(nl), true
		}
	}
	return line, false
}

// passiveTarget 返回被动模式数据连接在内网一侧的真实目标：
// 地址用控制通道的隧道目标地址（PASV 通告的内网地址经常是 0.0.0.0 或另一块网卡），
// 端口用 FTP 服务自己通告的端口。
func (s *ftpSession) passiveTarget(port int) string {
	if s.targetHost == "" || port <= 0 {
		return ""
	}
	return net.JoinHostPort(s.targetHost, strconv.Itoa(port))
}

// openRelay 在指定端口范围内开一个一次性中继端口，返回对外的端口号。
// target 在真正有数据连接进来时才求值，避免被动模式下端口还没确定就先算地址。
func (s *ftpSession) openRelay(spec string, target func() string) (int, error) {
	if s.publicIP == "" {
		return 0, errors.New("no public ip to advertise")
	}
	ln, err := listenFtpRelay(spec)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = ln.Close()
		return 0, errors.New("ftp session closed")
	}
	s.relays = append(s.relays, ln)
	// 超出上限时关掉最早的中继端口（FTP 客户端每次传输都会重新协商）
	for len(s.relays) > ftpMaxRelayPerSess {
		old := s.relays[0]
		s.relays = s.relays[1:]
		_ = old.Close()
	}
	s.mu.Unlock()
	go s.serveRelay(ln, target)
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// serveRelay 等一条数据连接进来，交给隧道的内网一侧，然后关掉这个中继端口
func (s *ftpSession) serveRelay(ln net.Listener, target func() string) {
	defer func() {
		_ = ln.Close()
		s.dropRelay(ln)
	}()
	tcpLn, ok := ln.(*net.TCPListener)
	if ok {
		// 没人连进来就把端口收回，避免临时端口被长期占用
		_ = tcpLn.SetDeadline(time.Now().Add(ftpRelayIdle()))
	}
	dc, err := ln.Accept()
	if ok {
		_ = tcpLn.SetDeadline(time.Time{})
	}
	if err != nil {
		return
	}
	addr := target()
	if addr == "" {
		logs.Warn("ftp port %d: data channel has no target address", s.srv.task.Port)
		_ = dc.Close()
		return
	}
	s.forwardData(dc, addr)
}

func (s *ftpSession) dropRelay(ln net.Listener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.relays {
		if v == ln {
			s.relays = append(s.relays[:i], s.relays[i+1:]...)
			return
		}
	}
}

// forwardData 把数据连接经隧道转到内网/客户端一侧
func (s *ftpSession) forwardData(dc net.Conn, addr string) {
	defer dc.Close()
	client := s.srv.task.Client
	if client == nil {
		return
	}
	if err := s.srv.CheckFlowAndConnNum(client); err != nil {
		logs.Warn("client id %d, task id %d, ftp data channel rejected: %s", client.Id, s.srv.task.Id, err.Error())
		return
	}
	defer client.AddConn()
	link := conn.NewLink(common.CONN_TCP, addr, client.Cnf.Crypt, client.Cnf.Compress, dc.RemoteAddr().String(), s.srv.task.Target.LocalProxy, conn.LinkTimeout(ftpDataDialTimeout))
	target, err := s.srv.bridge.SendLinkInfo(client.Id, link, s.srv.task)
	if err != nil {
		// 主动模式下这里失败是常态：客户端在内网，它通告的地址从内网一侧也连不到
		logs.Warn("ftp data channel to %s failed: %s", addr, err.Error())
		return
	}
	conn.CopyWaitGroup(target, dc, link.Crypt, link.Compress, client.Rate, s.srv.task.Flow, true, nil)
}

// listenFtpRelay 按端口范围配置监听一个中继端口，范围内都被占用或未配置时退回随机端口
func listenFtpRelay(spec string) (net.Listener, error) {
	for _, p := range ftp.ParsePortsSpec(spec) {
		if ln, err := net.Listen("tcp", ":"+strconv.Itoa(p)); err == nil {
			return ln, nil
		}
	}
	return net.Listen("tcp", ":0")
}

func ftpRelayIdle() time.Duration {
	sec := beego.AppConfig.DefaultInt("ftp_relay_timeout", int(ftpRelayDefaultIdle/time.Second))
	if sec <= 0 {
		return ftpRelayDefaultIdle
	}
	return time.Duration(sec) * time.Second
}

func ftpPasvPortSpec() string {
	return beego.AppConfig.String("ftp_pasv_ports")
}

func ftpActivePortSpec() string {
	// 主动模式的数据连接由 FTP 服务发起，源端口固定为 20，中继端口用独立范围便于区分
	if spec := beego.AppConfig.String("ftp_active_ports"); spec != "" {
		return spec
	}
	return ftpPasvPortSpec()
}
