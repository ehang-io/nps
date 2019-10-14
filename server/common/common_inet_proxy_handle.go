package common

import (
	"context"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/core"
	"net"
)

type Proxy struct {
	clientConn net.Conn
	ctx        context.Context
}

func (proxy *Proxy) GetConfigName() *core.NpsConfigs {
	return core.NewNpsConfigs("socks5_proxy", "proxy to inet")
}
func (proxy *Proxy) GetStage() core.Stage {
	return core.STAGE_RUN
}

func (proxy *Proxy) Start(ctx context.Context, config map[string]string) error {
	return nil
}

func (proxy *Proxy) Run(ctx context.Context, config map[string]string) error {
	clientCtxConn := ctx.Value(core.CLIENT_CONNECTION)
	if clientCtxConn == nil {
		return core.CLIENT_CONNECTION_NOT_EXIST
	}
	proxy.clientConn = clientCtxConn.(net.Conn)
	proxy.ctx = ctx
	bg := ctx.Value(core.BRIDGE)
	if bg == nil {
		return core.BRIDGE_NOT_EXIST
	}
	clientCtxConn := ctx.Value(core.CLIENT_CONNECTION)
	if clientCtxConn == nil {
		return core.CLIENT_CONNECTION_NOT_EXIST
	}

	clientId := ctx.Value(core.CLIENT_ID)
	if clientId == nil {
		return core.CLIENT_ID_NOT_EXIST
	}

	brg := bg.(*bridge.Bridge)
	severConn, err := brg.GetConnByClientId(clientId.(int))
	if err != nil {
		return err
	}
	go core.CopyBuffer(severConn, clientCtxConn.(net.Conn))
	core.CopyBuffer(clientCtxConn.(net.Conn), severConn)
	return nil
}

func (proxy *Proxy) End(ctx context.Context, config map[string]string) error {
	return nil
}
