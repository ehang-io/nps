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
	return core.NewNpsConfigs("socks5_proxy", "proxy to inet")
}

func (proxy *Proxy) Run(ctx context.Context, config map[string]string) (context.Context, error) {
	proxy.ctx = ctx
	proxy.clientConn = proxy.GetClientConn(ctx)
	clientId := proxy.GetClientId(ctx)
	brg := proxy.GetBridge(ctx)

	severConn, err := brg.GetConnByClientId(clientId)
	if err != nil {
		return ctx, err
	}

	go core.CopyBuffer(severConn, proxy.clientConn)
	core.CopyBuffer(proxy.clientConn, severConn)
	return ctx, nil
}
