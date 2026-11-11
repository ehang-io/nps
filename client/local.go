package client

import (
	"ehang.io/nps-mux"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/proxy"
	"github.com/astaxie/beego/logs"
	"github.com/xtaci/kcp-go"
)

var (
	LocalServer   []*net.TCPListener
	udpConn       net.Conn
	muxSession    *nps_mux.Mux
	fileServer    []*http.Server
	p2pNetBridge  *p2pBridge
	lock          sync.RWMutex
	udpConnStatus int32 // 原子读写：1 表示 p2p udp 通道可用
)

func isUdpReady() bool {
	return atomic.LoadInt32(&udpConnStatus) == 1
}

func setUdpReady(ok bool) {
	var v int32
	if ok {
		v = 1
	}
	atomic.StoreInt32(&udpConnStatus, v)
}

type p2pBridge struct {
}

func (p2pBridge *p2pBridge) SendLinkInfo(clientId int, link *conn.Link, t *file.Tunnel) (target net.Conn, err error) {
	for i := 0; muxSession == nil; i++ {
		if i >= 20 {
			err = errors.New("p2pBridge:too many times to get muxSession")
			logs.Error(err)
			return
		}
		runtime.Gosched() // waiting for another goroutine establish the mux connection
	}
	nowConn, err := muxSession.NewConn()
	if err != nil {
		udpConn = nil
		return nil, err
	}
	if _, err := conn.NewConn(nowConn).SendInfo(link, ""); err != nil {
		setUdpReady(false)
		return nil, err
	}
	return nowConn, nil
}

func CloseLocalServer() {
	lock.Lock()
	defer lock.Unlock()
	for _, v := range LocalServer {
		_ = v.Close()
	}
	for _, v := range fileServer {
		_ = v.Close()
	}
	LocalServer = nil
	fileServer = nil
}

func startLocalFileServer(config *config.CommonConfig, t *file.Tunnel, vkey, serverAddr string) *http.Server {
	if serverAddr == "" {
		serverAddr = config.Server
	}
	remoteConn, err := NewConn(config.Tp, vkey, serverAddr, common.WORK_FILE, config.ProxyUrl)
	if err != nil {
		logs.Error("Local connection server failed ", err.Error())
		return nil
	}
	srv := &http.Server{
		Handler: http.StripPrefix(t.StripPre, http.FileServer(http.Dir(t.LocalPath))),
	}
	logs.Info("start local file system, local path %s, strip prefix %s ,remote port %s ,server %s", t.LocalPath, t.StripPre, t.Ports, serverAddr)
	listener := nps_mux.NewMux(remoteConn.Conn, common.CONN_TCP, config.DisconnectTime)
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logs.Error(err)
		}
	}()
	return srv
}

func StartLocalServer(l *config.LocalServer, config *config.CommonConfig) error {
	if l.Type != "secret" {
		go handleUdpMonitor(config, l)
	}
	task := &file.Tunnel{
		Port:     l.Port,
		ServerIp: "0.0.0.0",
		Status:   true,
		Client: &file.Client{
			Cnf: &file.Config{
				U:        "",
				P:        "",
				Compress: config.Client.Cnf.Compress,
			},
			Status:    true,
			RateLimit: 0,
			Flow:      &file.Flow{},
		},
		Flow:   &file.Flow{},
		Target: &file.Target{},
	}
	switch l.Type {
	case "p2ps":
		logs.Info("successful start-up of local socks5 monitoring, port", l.Port)
		return proxy.NewSock5ModeServer(p2pNetBridge, task).Start()
	case "p2pt":
		logs.Info("successful start-up of local tcp trans monitoring, port", l.Port)
		return proxy.NewTunnelModeServer(proxy.HandleTrans, p2pNetBridge, task).Start()
	case "p2p", "secret":
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: l.Port})
		if err != nil {
			logs.Error("local listener startup failed port %d, error %s", l.Port, err.Error())
			return err
		}
		LocalServer = append(LocalServer, listener)
		logs.Info("successful start-up of local tcp monitoring, port", l.Port)
		conn.Accept(listener, func(c net.Conn) {
			logs.Trace("new %s connection", l.Type)
			if l.Type == "secret" {
				handleSecret(c, config, l)
			} else if l.Type == "p2p" {
				handleP2PVisitor(c, config, l)
			}
		})
	}
	return nil
}

func handleUdpMonitor(config *config.CommonConfig, l *config.LocalServer) {
	// 原先每秒固定尝试 10 次且每次失败都打 Error，通道长期不可用时会把日志和连接数刷爆。
	// 现在每轮只尝试一次，按 1s→2s→4s…退避，同一个错误只提示一次。
	backoff := time.Second
	var lastErr string
	report := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if msg == lastErr {
			return
		}
		lastErr = msg
		logs.Error("%s", msg)
	}
	for {
		if isUdpReady() {
			time.Sleep(time.Second)
			lastErr = ""
			continue
		}
		lock.Lock()
		udpConn = nil
		lock.Unlock()
		tmpConn, err := common.GetLocalUdpAddr()
		if err != nil {
			report("p2p: allocate local udp address failed: %s", err.Error())
			time.Sleep(backoff)
			continue
		}
		localAddr := tmpConn.LocalAddr().String()
		// 只借这个 socket 取一个空闲端口，取完立刻释放，否则每轮重试都漏一个 FD
		_ = tmpConn.Close()
		if err = newUdpConn(localAddr, config, l); err == nil && isUdpReady() {
			backoff = time.Second
			lastErr = ""
			continue
		}
		if errors.Is(err, errP2PAddrInvalid) {
			// 服务端没启用 p2p 时重试不会自愈：退到 60s 且只提示一次，
			// 具体要改哪些配置已写在错误信息里
			report("p2p disabled on server, traffic falls back through the server: %s", err.Error())
			time.Sleep(time.Minute)
			continue
		}
		if err != nil {
			report("p2p udp channel setup failed: %s", err.Error())
		} else {
			report("p2p udp channel not ready after setup")
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func handleSecret(localTcpConn net.Conn, config *config.CommonConfig, l *config.LocalServer) {
	remoteConn, err := NewConn(config.Tp, config.VKey, config.Server, common.WORK_SECRET, config.ProxyUrl)
	if err != nil {
		logs.Error("Local connection server failed ", err.Error())
		return
	}
	if _, err := remoteConn.Write([]byte(crypt.Md5(l.Password))); err != nil {
		logs.Error("Local connection server failed ", err.Error())
		return
	}
	conn.CopyWaitGroup(remoteConn.Conn, localTcpConn, false, false, nil, nil, false, nil)
}

func handleP2PVisitor(localTcpConn net.Conn, config *config.CommonConfig, l *config.LocalServer) {
	lock.RLock()
	ready := udpConn != nil
	lock.RUnlock()
	if !ready {
		logs.Notice("new conn, P2P can not penetrate successfully, traffic will be transferred through the server")
		handleSecret(localTcpConn, config, l)
		return
	}
	logs.Trace("start trying to connect with the server")
	//TODO just support compress now because there is not tls file in client packages
	link := conn.NewLink(common.CONN_TCP, l.Target, false, config.Client.Cnf.Compress, localTcpConn.LocalAddr().String(), false)
	if target, err := p2pNetBridge.SendLinkInfo(0, link, nil); err != nil {
		logs.Error(err)
		setUdpReady(false)
		return
	} else {
		conn.CopyWaitGroup(target, localTcpConn, false, config.Client.Cnf.Compress, nil, nil, false, nil)
	}
}

func newUdpConn(localAddr string, config *config.CommonConfig, l *config.LocalServer) error {
	// 连接建立涉及多次网络往返，不再持有全局锁，避免阻塞 p2p 转发路径上的读锁。
	remoteConn, err := NewConn(config.Tp, config.VKey, config.Server, common.WORK_P2P, config.ProxyUrl)
	if err != nil {
		return fmt.Errorf("connect to server %s: %w", config.Server, err)
	}
	// 失败路径必须关掉这条控制连接，否则每轮重试都漏一条 TCP 连接
	defer func() { _ = remoteConn.Close() }()
	if _, err := remoteConn.Write([]byte(crypt.Md5(l.Password))); err != nil {
		return fmt.Errorf("send p2p secret to %s: %w", config.Server, err)
	}
	var rAddr []byte
	//读取服务端地址、密钥 继续做处理
	if rAddr, err = remoteConn.GetShortLenContent(); err != nil {
		return fmt.Errorf("read p2p address from %s: %w", config.Server, err)
	}
	var localConn net.PacketConn
	var remoteAddress string
	if remoteAddress, localConn, err = handleP2PUdp(localAddr, string(rAddr), crypt.Md5(l.Password), common.WORK_P2P_VISITOR); err != nil {
		return err
	}
	udpTunnel, err := kcp.NewConn(remoteAddress, nil, 150, 3, localConn)
	if err != nil {
		return fmt.Errorf("create kcp connection to %s: %w", remoteAddress, err)
	}
	if udpTunnel == nil {
		return fmt.Errorf("create kcp connection to %s: no session", remoteAddress)
	}
	logs.Trace("successful create a connection with server", remoteAddress)
	conn.SetUdpSession(udpTunnel)
	lock.Lock()
	udpConn = udpTunnel
	muxSession = nps_mux.NewMux(udpConn, "kcp", config.DisconnectTime)
	p2pNetBridge = &p2pBridge{}
	lock.Unlock()
	setUdpReady(true)
	return nil
}
