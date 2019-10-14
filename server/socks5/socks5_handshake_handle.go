package socks5

import (
	"context"
	"errors"
	"fmt"
	"github.com/cnlh/nps/core"
	"io"
)

type Handshake struct {
	core.NpsPlugin
}

func (handshake *Handshake) Run(ctx context.Context, config map[string]string) error {
	clientConn := handshake.GetClientConn(ctx)
	buf := make([]byte, 2)
	if _, err := io.ReadFull(clientConn, buf); err != nil {
		return errors.New("negotiation err while read 2 bytes from client connection: " + err.Error())
	}

	if version := buf[0]; version != 5 {
		return errors.New("only support socks5")
	}
	nMethods := buf[1]

	methods := make([]byte, nMethods)

	if n, err := clientConn.Read(methods); n != int(nMethods) || err != nil {
		return errors.New(fmt.Sprintf("read methods error, need %d , read  %d, error %s", nMethods, n, err.Error()))
	} else {
		context.WithValue(ctx, "methods", methods[:n])
	}

	return nil
}
