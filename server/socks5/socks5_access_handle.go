package socks5

import (
	"context"
	"errors"
	"github.com/cnlh/nps/core"
	"io"
	"net"
)

const (
	UserPassAuth    = uint8(2)
	userAuthVersion = uint8(1)
	authSuccess     = uint8(0)
	authFailure     = uint8(1)
	UserNoAuth      = uint8(0)
)

type Access struct {
	clientConn net.Conn
	username   string
	password   string
}

func (access *Access) GetConfigName() []*core.Config {
	c := make([]*core.Config, 0)
	c = append(c, &core.Config{ConfigName: "socks5_check_access", Description: "need check the permission?"})
	c = append(c, &core.Config{ConfigName: "socks5_access_username", Description: "auth username"})
	c = append(c, &core.Config{ConfigName: "socks5_access_password", Description: "auth password"})
	return nil
}

func (access *Access) GetStage() core.Stage {
	return core.STAGE_RUN
}

func (access *Access) GetBeforePlugin() core.Plugin {
	return &Handshake{}
}

func (access *Access) Start(ctx context.Context, config map[string]string) error {
	return nil
}
func (access *Access) End(ctx context.Context, config map[string]string) error {
	return nil
}

func (access *Access) Run(ctx context.Context, config map[string]string) error {
	clientCtxConn := ctx.Value("clientConn")
	if clientCtxConn == nil {
		return errors.New("the client access.clientConnection is not exist")
	}
	access.clientConn = clientCtxConn.(net.Conn)
	if config["socks5_check_access"] != "true" {
		return access.sendAccessMsgToClient(UserNoAuth)
	}
	configUsername := config["socks5_access_username"]
	configPassword := config["socks5_access_password"]
	if configUsername == "" || configPassword == "" {
		return access.sendAccessMsgToClient(UserNoAuth)
	}
	// need auth
	if err := access.sendAccessMsgToClient(UserPassAuth); err != nil {
		return err
	}
	// send auth reply to client ,and get the auth information
	var err error
	access.username, access.password, err = access.getAuthInfoFromClient()
	if err != nil {
		return err
	}
	context.WithValue(ctx, access.username, access.password)
	// check
	return access.checkAuth(configUsername, configPassword)
}

func (access *Access) sendAccessMsgToClient(auth uint8) error {
	buf := make([]byte, 2)
	buf[0] = 5
	buf[1] = auth
	n, err := access.clientConn.Write(buf)
	if err != nil || n != 2 {
		return errors.New("write access message to client error " + err.Error())
	}
	return nil
}

func (access *Access) getAuthInfoFromClient() (username string, password string, err error) {
	header := []byte{0, 0}
	if _, err = io.ReadAtLeast(access.clientConn, header, 2); err != nil {
		return
	}
	if header[0] != userAuthVersion {
		err = errors.New("authentication method is not supported")
		return
	}
	userLen := int(header[1])
	user := make([]byte, userLen)
	if _, err = io.ReadAtLeast(access.clientConn, user, userLen); err != nil {
		return
	}
	if _, err := access.clientConn.Read(header[:1]); err != nil {
		err = errors.New("get password length error" + err.Error())
		return
	}
	passLen := int(header[0])
	pass := make([]byte, passLen)
	if _, err := io.ReadAtLeast(access.clientConn, pass, passLen); err != nil {
		err = errors.New("get password error" + err.Error())
		return
	}
	username = string(user)
	password = string(pass)
	return
}

func (access *Access) checkAuth(configUserName, configPassword string) error {
	if access.username == configUserName && access.password == configPassword {
		_, err := access.clientConn.Write([]byte{userAuthVersion, authSuccess})
		return err
	} else {
		_, err := access.clientConn.Write([]byte{userAuthVersion, authFailure})
		if err != nil {
			return err
		}
		return errors.New("auth check error,username or password does not match")
	}
}
