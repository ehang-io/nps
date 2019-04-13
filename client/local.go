package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/mux"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"net"
	"net/http"
	"sync"
)

var (
	LocalServer []*net.TCPListener
	udpConn     net.Conn
	muxSession  *mux.Mux
	fileServer  []*http.Server
	lock        sync.Mutex
	hasP2PTry   bool
)

func CloseLocalServer() {
	for _, v := range LocalServer {
		v.Close()
	}
	for _, v := range fileServer {
		v.Close()
	}
}

func startLocalFileServer(config *config.CommonConfig, t *file.Tunnel, vkey string) {
	remoteConn, err := NewConn(config.Tp, vkey, config.Server, common.WORK_FILE, config.ProxyUrl)
	if err != nil {
		logs.Error("Local connection server failed ", err.Error())
		return
	}
	srv := &http.Server{
		Handler: http.StripPrefix(t.StripPre, http.FileServer(http.Dir(t.LocalPath))),
	}
	logs.Info("start local file system, local path %s, strip prefix %s ,remote port %s ", t.LocalPath, t.StripPre, t.Ports)
	fileServer = append(fileServer, srv)
	listener := mux.NewMux(remoteConn.Conn, common.CONN_TCP)
	logs.Error(srv.Serve(listener))
}

func StartLocalServer(l *config.LocalServer, config *config.CommonConfig) error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), l.Port, ""})
	if err != nil {
		logs.Error("local listener startup failed port %d, error %s", l.Port, err.Error())
		return err
	}
	LocalServer = append(LocalServer, listener)
	logs.Info("successful start-up of local monitoring, port", l.Port)
	conn.Accept(listener, func(c net.Conn) {
		if l.Type == "secret" {
			handleSecret(c, config, l)
		} else {
			handleP2PVisitor(c, config, l)
		}
	})
	return nil
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
restart:
	lock.Lock()
	if udpConn == nil {
		if !hasP2PTry {
			hasP2PTry = true
			newUdpConn(config, l)
		}
		if udpConn == nil {
			lock.Unlock()
			logs.Notice("new conn, P2P can not penetrate successfully, traffic will be transferred through the server")
			handleSecret(localTcpConn, config, l)
			return
		} else {
			muxSession = mux.NewMux(udpConn, "kcp")
		}
	}
	lock.Unlock()
	logs.Trace("start trying to connect with the server")
	nowConn, err := muxSession.NewConn()
	if err != nil {
		udpConn = nil
		logs.Error(err, "reconnect......")
		goto restart
		return
	}
	//TODO just support compress now because there is not tls file in client packages
	link := conn.NewLink(common.CONN_TCP, l.Target, false, config.Client.Cnf.Compress, localTcpConn.LocalAddr().String(), false)
	if _, err := conn.NewConn(nowConn).SendInfo(link, ""); err != nil {
		logs.Error(err)
		return
	}
	conn.CopyWaitGroup(nowConn, localTcpConn, false, config.Client.Cnf.Compress, nil, nil, false, nil)
}

func newUdpConn(config *config.CommonConfig, l *config.LocalServer) {
	remoteConn, err := NewConn(config.Tp, config.VKey, config.Server, common.WORK_P2P, config.ProxyUrl)
	if err != nil {
		logs.Error("Local connection server failed ", err.Error())
		return
	}
	if _, err := remoteConn.Write([]byte(crypt.Md5(l.Password))); err != nil {
		logs.Error("Local connection server failed ", err.Error())
		return
	}
	var rAddr []byte
	//读取服务端地址、密钥 继续做处理
	if rAddr, err = remoteConn.GetShortLenContent(); err != nil {
		logs.Error(err)
		return
	}
	var localConn net.PacketConn
	var remoteAddress string
	if remoteAddress, localConn, err = handleP2PUdp(string(rAddr), crypt.Md5(l.Password), common.WORK_P2P_VISITOR); err != nil {
		logs.Error(err)
		return
	}
	udpTunnel, err := kcp.NewConn(remoteAddress, nil, 150, 3, localConn)
	if err != nil || udpTunnel == nil {
		logs.Warn(err)
		return
	}
	logs.Trace("successful create a connection with server", remoteAddress)
	conn.SetUdpSession(udpTunnel)
	udpConn = udpTunnel
}
