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
	clientConn net.Conn
}

func (access *Access) GetConfigName() *core.NpsConfigs {
	return core.NewNpsConfigs("socks5_check_access_check", "need check the permission simply")
}

func (access *Access) Run(ctx context.Context, config map[string]string) error {
	access.clientConn = access.GetClientConn(ctx)
	if config["socks5_check_access"] != "true" {
		return access.sendAccessMsgToClient(UserNoAuth)
	}
	// need auth
	if err := access.sendAccessMsgToClient(UserPassAuth); err != nil {
		return err
	}
	// send auth reply to client ,and get the auth information
	username, password, err := access.getAuthInfoFromClient()
	if err != nil {
		return err
	}
	context.WithValue(ctx, "socks_client_username", username)
	context.WithValue(ctx, "socks_client_password", password)
	// check
	return nil
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
