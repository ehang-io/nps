package socks5

import (
	"github.com/cnlh/nps/core"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/server/common"
	"net"
	"strconv"
)

type S5Server struct {
	globalConfig map[string]string
	clientConfig map[string]string
	pluginConfig map[string]string
	ServerIp     string
	ServerPort   int
	plugins      *core.Plugins
	listener     net.Listener
}

func NewS5Server(globalConfig, clientConfig, pluginConfig map[string]string) *S5Server {
	s5 := &S5Server{
		globalConfig: globalConfig,
		clientConfig: clientConfig,
		pluginConfig: pluginConfig,
		plugins:      &core.Plugins{},
	}
	s5.plugins.Add(new(Handshake), new(Access), new(CheckAccess), new(Request), new(common.Proxy))
	return s5
}

func (s5 *S5Server) Start() error {
	return conn.NewTcpListenerAndProcess(s5.ServerIp+":"+strconv.Itoa(s5.ServerPort), func(c net.Conn) {

	}, &s5.listener)
}
