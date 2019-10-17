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
	core.NpsPlugin
}

func (access *Access) GetConfigName() *core.NpsConfigs {
	return core.NewNpsConfigs("socks5_check_access", "need check the permission simply", core.CONFIG_LEVEL_PLUGIN)
}

func (access *Access) Run(ctx context.Context) (context.Context, error) {
	clientConn := access.GetClientConn(ctx)
	if access.Configs["socks5_check_access"] != "true" {
		return ctx, access.sendAccessMsgToClient(clientConn, UserNoAuth)
	}
	// need auth
	if err := access.sendAccessMsgToClient(clientConn, UserPassAuth); err != nil {
		return ctx, err
	}
	// send auth reply to client ,and get the auth information
	username, password, err := access.getAuthInfoFromClient(clientConn)
	if err != nil {
		return ctx, err
	}
	ctx = context.WithValue(ctx, "socks_client_username", username)
	ctx = context.WithValue(ctx, "socks_client_password", password)
	// check
	return ctx, nil
}

func (access *Access) sendAccessMsgToClient(clientConn net.Conn, auth uint8) error {
	buf := make([]byte, 2)
	buf[0] = 5
	buf[1] = auth
	n, err := clientConn.Write(buf)
	if err != nil || n != 2 {
		return errors.New("write access message to client error " + err.Error())
	}
	return nil
}

func (access *Access) getAuthInfoFromClient(clientConn net.Conn) (username string, password string, err error) {
	header := []byte{0, 0}
	if _, err = io.ReadAtLeast(clientConn, header, 2); err != nil {
		return
	}
	if header[0] != userAuthVersion {
		err = errors.New("authentication method is not supported")
		return
	}
	userLen := int(header[1])
	user := make([]byte, userLen)
	if _, err = io.ReadAtLeast(clientConn, user, userLen); err != nil {
		return
	}
	if _, err = clientConn.Read(header[:1]); err != nil {
		err = errors.New("get password length error" + err.Error())
		return
	}
	passLen := int(header[0])
	pass := make([]byte, passLen)
	if _, err = io.ReadAtLeast(clientConn, pass, passLen); err != nil {
		err = errors.New("get password error" + err.Error())
		return
	}
	username = string(user)
	password = string(pass)
	return
}
