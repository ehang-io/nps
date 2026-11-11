package proxy

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/connection"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type TunnelModeServer struct {
	BaseServer
	process  process
	listener net.Listener
	guard    *common.AccessGuard
}

//tcp|http|host
func NewTunnelModeServer(process process, bridge NetBridge, task *file.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	return s
}

//开始
func (s *TunnelModeServer) Start() error {
	s.guard = TaskGuard(s.task)
	if s.guard != nil && !s.guard.Disabled() {
		logs.Info(guardDescription(s.guard, s.task.Port))
	}
	if (s.task.Mode == "tcp" || s.task.Mode == "ftp") && (s.task.TlsEnabled() || s.task.AuthEnabled()) {
		logs.Info("port %d tcp tunnel protection enabled, tls=%v auth_user=%q", s.task.Port, s.task.TlsEnabled(), s.task.AuthUser)
	}
	return conn.NewTcpListenerAndProcess(s.task.ServerIp+":"+strconv.Itoa(s.task.Port), func(c net.Conn) {
		//公网侧连接同样关闭 Nagle、开启保活，并按端口画像设置失效连接回收时间：
		//3389/22 这类会话靠的就是小包及时下发，且对端消失后要尽快释放连接
		c = common.TuneConnForPort(c, s.task.Port)
		//来源校验必须在读取任何字节、建立隧道之前完成：被拒绝的扫描器
		//拿不到目标服务的 banner，请求也不会转发到内网（3389 被爆破会锁账号）
		if ok, reason := s.guard.Check(c.RemoteAddr()); !ok {
			logReject(c.RemoteAddr(), reason)
			rejectConn(c)
			return
		}
		//TLS 与 Basic 认证同样在建立隧道之前完成：未通过认证的连接既不占用客户端连接数，
		//也不会触达内网目标。认证帧读到的多余字节通过 preReadConn 回放给目标。
		wrapped, err := wrapTunnelConn(c, s.task)
		if err != nil {
			if !errors.Is(err, errAuthResponded) {
				if errors.Is(err, errAuthFailed) || errors.Is(err, errAuthTooLarge) {
					logReject(c.RemoteAddr(), "tcp tunnel auth rejected")
				}
				rejectConn(c)
			}
			return
		}
		//首包守卫：RDP/SSH/HTTP 都是客户端先说话，连上却一直不发数据的连接是扫描器，
		//在建立隧道之前就关掉，既省资源也不把端口状态暴露给探测脚本。
		//已在上面完成 TLS/认证握手的连接无需再等（握手本身已经消费了首包）。
		//FTP 是服务端先发 220 欢迎语，等客户端首包只会把正常用户挡在门外，所以一律跳过。
		if !s.task.TlsEnabled() && !s.task.AuthEnabled() && !FtpAlgForTunnel(s.task) {
			guarded, gerr := common.AwaitFirstByte(wrapped, FirstByteGuardTimeout(s.task))
			if gerr != nil {
				logReject(c.RemoteAddr(), "no application data after connect")
				rejectConn(wrapped)
				return
			}
			wrapped = guarded
		}
		if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
			logs.Warn("client id %d, task id %d,error %s, when tcp connection", s.task.Client.Id, s.task.Id, err.Error())
			_ = wrapped.Close()
			return
		}
		logs.Trace("new tcp connection,local port %d,client %d,remote address %s", s.task.Port, s.task.Client.Id, c.RemoteAddr())
		// give the counter back even if the proxy panics, otherwise the client
		// slot leaks until nps restarts
		defer s.task.Client.AddConn()
		s.process(conn.NewConn(wrapped), s)
	}, &s.listener)
}

//close
func (s *TunnelModeServer) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

//web管理方式
type WebServer struct {
	BaseServer
}

//开始
func (s *WebServer) Start() error {
	p, _ := beego.AppConfig.Int("web_port")
	if p == 0 {
		stop := make(chan struct{})
		<-stop
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.SetStaticPath(beego.AppConfig.String("web_base_url")+"/static", filepath.Join(common.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(common.GetRunPath(), "web", "views"))
	err := errors.New("Web management startup failure ")
	var l net.Listener
	if l, err = connection.GetWebManagerListener(); err == nil {
		beego.InitBeforeHTTPRun()
		if beego.AppConfig.String("web_open_ssl") == "true" {
			keyPath := beego.AppConfig.String("web_key_file")
			certPath := beego.AppConfig.String("web_cert_file")
			err = http.ServeTLS(l, beego.BeeApp.Handlers, certPath, keyPath)
		} else {
			err = http.Serve(l, beego.BeeApp.Handlers)
		}
	} else {
		logs.Error(err)
	}
	return err
}

func (s *WebServer) Close() error {
	return nil
}

//new
func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

type process func(c *conn.Conn, s *TunnelModeServer) error

//tcp proxy
func ProcessTunnel(c *conn.Conn, s *TunnelModeServer) error {
	targetAddr, err := s.task.Target.GetRandomTarget()
	if err != nil {
		c.Close()
		logs.Warn("tcp port %d ,client id %d,task id %d connect error %s", s.task.Port, s.task.Client.Id, s.task.Id, err.Error())
		return err
	}
	return s.DealClient(c, s.task.Client, targetAddr, nil, common.CONN_TCP, nil, s.task.Flow, s.task.Target.LocalProxy)
}

//http proxy
func ProcessHttp(c *conn.Conn, s *TunnelModeServer) error {
	_, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		logs.Info(err)
		return err
	}
	if r.Method == "CONNECT" {
		c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		rb = nil
	}
	if err := s.auth(r, c, s.task.Client.Cnf.U, s.task.Client.Cnf.P); err != nil {
		return err
	}
	return s.DealClient(c, s.task.Client, addr, rb, common.CONN_TCP, nil, s.task.Flow, s.task.Target.LocalProxy)
}
