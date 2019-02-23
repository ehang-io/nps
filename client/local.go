package client

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
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

func process(conn net.Conn, config *config.CommonConfig, l *config.LocalServer) {
	c, err := NewConn(config.Tp, config.VKey, config.Server, common.WORD_SECRET, config.ProxyUrl)
	if err != nil {
		logs.Error("Local connection server failed ", err.Error())
	}
	if _, err := c.Write([]byte(crypt.Md5(l.Password))); err != nil {
		logs.Error("Local connection server failed ", err.Error())
	}
	go common.CopyBuffer(c, conn)
	common.CopyBuffer(conn, c)
	c.Close()
	conn.Close()
}
