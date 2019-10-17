package socks5

import (
	"context"
	"fmt"
	"github.com/cnlh/nps/core"
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

func NewS5Server(globalConfig, clientConfig, pluginConfig map[string]string, ServerIp string, ServerPort int) *S5Server {
	s5 := &S5Server{
		globalConfig: globalConfig,
		clientConfig: clientConfig,
		pluginConfig: pluginConfig,
		plugins:      &core.Plugins{},
		ServerIp:     ServerIp,
		ServerPort:   ServerPort,
	}
	s5.plugins.Add(new(Handshake), new(Access), new(CheckAccess), new(Request), new(common.Proxy))
	return s5
}

func (s5 *S5Server) Start(ctx context.Context) error {
	// init config of plugin
	for _, pg := range s5.plugins.AllPgs {
		if pg.GetConfigName() != nil {
			pg.InitConfig(s5.globalConfig, s5.clientConfig, s5.pluginConfig, pg.GetConfigName().GetAll())
		}
	}

	core.NewTcpListenerAndProcess(s5.ServerIp+":"+strconv.Itoa(s5.ServerPort), func(c net.Conn) {
		// init ctx value clientConn
		connCtx := context.WithValue(ctx, core.CLIENT_CONNECTION, c)
		var err error

		// run the plugin contains start
		if connCtx, err = core.RunPlugin(connCtx, s5.plugins.StartPgs, core.STAGE_START); err != nil {
			fmt.Println(err)
			return
		}

		// start run the plugin run
		if connCtx, err = core.RunPlugin(connCtx, s5.plugins.RunPgs, core.STAGE_RUN); err != nil {
			fmt.Println(err)
			return
		}

		// start run the plugin end
		if connCtx, err = core.RunPlugin(connCtx, s5.plugins.EndPgs, core.STAGE_END); err != nil {
			fmt.Println(err)
			return
		}
	}, &s5.listener)
	return nil
}
