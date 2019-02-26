package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"net"
	"strings"
)

var LocalServer []*net.TCPListener

func CloseLocalServer() {
	for _, v := range LocalServer {
		v.Close()
	}
}

func StartLocalServer(l *config.LocalServer, config *config.CommonConfig) error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), l.Port, ""})
	if err != nil {
		logs.Error("Local listener startup failed port %d, error %s", l.Port, err.Error())
		return err
	}
	LocalServer = append(LocalServer, listener)
	logs.Info("Successful start-up of local monitoring, port", l.Port)
	for {
		c, err := listener.AcceptTCP()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			logs.Info(err)
			continue
		}
		go process(c, config, l)
	}
	return nil
}

func process(localTcpConn net.Conn, config *config.CommonConfig, l *config.LocalServer) {
	var workType string
	if l.Type == "secret" {
		workType = common.WORK_SECRET
	} else {
		workType = common.WORK_P2P
	}
	remoteConn, err := NewConn(config.Tp, config.VKey, config.Server, workType, config.ProxyUrl)
	if err != nil {
		logs.Error("Local connection server failed ", err.Error())
	}
	if _, err := remoteConn.Write([]byte(crypt.Md5(l.Password))); err != nil {
		logs.Error("Local connection server failed ", err.Error())
	}
	if l.Type == "secret" {
		go common.CopyBuffer(remoteConn, localTcpConn)
		common.CopyBuffer(localTcpConn, remoteConn)
		remoteConn.Close()
		localTcpConn.Close()
	} else {
		//读取服务端地址、密钥 继续做处理
		logs.Warn(111)
		if rAddr, err := remoteConn.GetLenContent(); err != nil {
			return
		} else {
			logs.Warn(222)
			//与服务端udp建立连接
			tmpConn, err := net.Dial("udp", "114.114.114.114:53")
			if err != nil {
				logs.Warn(err)
			}
			tmpConn.Close()
			//与服务端建立udp连接
			localAddr, _ := net.ResolveUDPAddr("udp", tmpConn.LocalAddr().String())
			localConn, err := net.ListenUDP("udp", localAddr)
			if err != nil {
				return
			}
			logs.Warn(333)
			localKcpConn, err := kcp.NewConn(string(rAddr), nil, 150, 3, localConn)
			conn.SetUdpSession(localKcpConn)
			if err != nil {
				logs.Warn(err)
			}
			localToolConn := conn.NewConn(localKcpConn)
			//写入密钥、provider身份
			if _, err := localToolConn.Write([]byte(crypt.Md5(l.Password))); err != nil {
				return
			}
			if _, err := localToolConn.Write([]byte(common.WORK_P2P_VISITOR)); err != nil {
				return
			}
			logs.Warn(444)
			//接收服务端传的visitor地址
			if b, err := localToolConn.GetLenContent(); err != nil {
				logs.Warn(err)
				return
			} else {
				logs.Warn("收到服务回传地址", string(b))
				//关闭与服务端连接
				localConn.Close()
				//建立新的连接
				localConn, err = net.ListenUDP("udp", localAddr)
				udpTunnel, err := kcp.NewConn(string(b), nil, 150, 3, localConn)
				if err != nil || udpTunnel == nil {
					logs.Warn(err)
					return
				}
				conn.SetUdpSession(udpTunnel)
				logs.Warn(udpTunnel.RemoteAddr(), string(b), udpTunnel.LocalAddr())

				go common.CopyBuffer(udpTunnel, localTcpConn)
				common.CopyBuffer(localTcpConn, udpTunnel)
			}
		}

	}
}
