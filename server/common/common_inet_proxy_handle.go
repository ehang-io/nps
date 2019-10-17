package common

import (
	"context"
	"fmt"
	"github.com/cnlh/nps/core"
	"net"
	"strconv"
)

type Proxy struct {
	core.NpsPlugin
}

func (proxy *Proxy) GetConfigName() *core.NpsConfigs {
	return core.NewNpsConfigs("socks5_proxy", "proxy to inet", core.CONFIG_LEVEL_PLUGIN)
}

func (proxy *Proxy) Run(ctx context.Context) (context.Context, error) {
	clientConn := proxy.GetClientConn(ctx)
	//clientId := proxy.GetClientId(ctx)
	//brg := proxy.GetBridge(ctx)

	//severConn, err := brg.GetConnByClientId(clientId)
	//if err != nil {
	//	return ctx, err
	//}
	//
	//// send connection information to the npc
	//if _, err := core.SendInfo(severConn, nil); err != nil {
	//	return ctx, err
	//}
	connType := ctx.Value(core.PROXY_CONNECTION_TYPE).(string)
	connAddr := ctx.Value(core.PROXY_CONNECTION_ADDR).(string)
	connPort := strconv.Itoa(int(ctx.Value(core.PROXY_CONNECTION_PORT).(uint16)))
	fmt.Println(connType, connAddr, connPort, clientConn.RemoteAddr().String())
	serverConn, err := net.Dial(connType, connAddr+":"+connPort)
	if err != nil {
		return ctx, err
	}
	// data exchange
	go func() {
		core.CopyBuffer(serverConn, clientConn)
		serverConn.Close()
		clientConn.Close()
	}()
	core.CopyBuffer(clientConn, serverConn)
	serverConn.Close()
	clientConn.Close()
	return ctx, core.REQUEST_EOF
}
