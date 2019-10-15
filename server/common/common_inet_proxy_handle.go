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

func (proxy *Proxy) Run(ctx context.Context, config map[string]string) error {
	proxy.clientConn = proxy.GetClientConn(ctx)
	proxy.ctx = ctx

	clientCtxConn := ctx.Value(core.CLIENT_CONNECTION)
	if clientCtxConn == nil {
		return core.CLIENT_CONNECTION_NOT_EXIST
	}

	clientId := proxy.GetClientId(ctx)

	brg := proxy.GetBridge(ctx)
	severConn, err := brg.GetConnByClientId(clientId)
	if err != nil {
		return err
	}

	go core.CopyBuffer(severConn, clientCtxConn.(net.Conn))
	core.CopyBuffer(clientCtxConn.(net.Conn), severConn)
	return nil
}
