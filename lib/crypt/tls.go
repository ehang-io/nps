package crypt

import (
	"crypto/tls"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"net"
	"os"
	"path/filepath"
)

func NewTlsServerConn(conn net.Conn) net.Conn {
	cert, err := tls.LoadX509KeyPair(filepath.Join(beego.AppPath, "conf", "server.pem"), filepath.Join(beego.AppPath, "conf", "server.key"))
	if err != nil {
		logs.Error(err)
		os.Exit(0)
		return nil
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	return tls.Server(conn, config)
}

func NewTlsClientConn(conn net.Conn) net.Conn {
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	return tls.Client(conn, conf)
}
