package crypt

import (
	"crypto/tls"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"net"
	"os"
)

var pemPath, keyPath string

func InitTls(pem, key string) {
	pemPath = pem
	keyPath = key
}

func NewTlsServerConn(conn net.Conn) net.Conn {
	cert, err := tls.LoadX509KeyPair(pemPath, keyPath)
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
