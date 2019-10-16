package common

import (
	"context"
	"github.com/cnlh/nps/core"
	"net"
)

type Proxy struct {
	core.NpsPlugin
	clientConn net.Conn
	ctx        context.Context
}

func (proxy *Proxy) GetConfigName() *core.NpsConfigs {
	return core.NewNpsConfigs("socks5_proxy", "proxy to inet", core.CONFIG_LEVEL_PLUGIN)
}

func (proxy *Proxy) Run(ctx context.Context) (context.Context, error) {
	proxy.ctx = ctx
	proxy.clientConn = proxy.GetClientConn(ctx)
	clientId := proxy.GetClientId(ctx)
	brg := proxy.GetBridge(ctx)

	//severConn, err := brg.GetConnByClientId(clientId)
	//if err != nil {
	//	return ctx, err
	//}
	//
	//// send connection information to the npc
	//if _, err := core.SendInfo(severConn, nil); err != nil {
	//	return ctx, err
	//}
	severConn, err := net.Dial(ctx.Value(core.PROXY_CONNECTION_TYPE).(string), ctx.Value(core.PROXY_CONNECTION_ADDR).(string)+":"+ctx.Value(core.PROXY_CONNECTION_PORT).(string))
	if err != nil {
		return ctx, err
	}
	// data exchange
	go core.CopyBuffer(severConn, proxy.clientConn)
	core.CopyBuffer(proxy.clientConn, severConn)
	return ctx, core.REQUEST_EOF
}
