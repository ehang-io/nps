package socks5

import (
	"context"
	"errors"
	"fmt"
	"github.com/cnlh/nps/core"
	"net"
)

type CheckAccess struct {
	core.NpsPlugin
	configUsername string
	configPassword string
}

func (check *CheckAccess) GetConfigName() *core.NpsConfigs {
	c := core.NewNpsConfigs("socks5_simple_access_check", "need check the permission simply", core.CONFIG_LEVEL_PLUGIN)
	c.Add("socks5_simple_access_username", "simple auth username", core.CONFIG_LEVEL_PLUGIN)
	c.Add("socks5_simple_access_password", "simple auth password", core.CONFIG_LEVEL_PLUGIN)
	return c
}

func (check *CheckAccess) Run(ctx context.Context) (context.Context, error) {
	clientConn := check.GetClientConn(ctx)
	check.configUsername = check.Configs["socks5_simple_access_username"]
	check.configPassword = check.Configs["socks5_simple_access_password"]
	if check.Configs["socks5_simple_access_check"] == "true" {
		connUsername := ctx.Value("socks_client_username").(string)
		connPassword := ctx.Value("socks_client_password").(string)
		return ctx, check.checkAuth(clientConn, connUsername, connPassword)
	}
	return ctx, nil
}

func (check *CheckAccess) checkAuth(clientConn net.Conn, connUserName, connPassword string) error {
	if check.configUsername == connUserName && check.configPassword == connPassword {
		_, err := clientConn.Write([]byte{userAuthVersion, authSuccess})
		return err
	} else {
		_, err := clientConn.Write([]byte{userAuthVersion, authFailure})
		if err != nil {
			return err
		}
		return errors.New("auth check error,username or password does not match")
	}
}
