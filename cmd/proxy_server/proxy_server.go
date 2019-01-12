package main

import (
	"flag"
	"github.com/astaxie/beego"
	"github.com/cnlh/easyProxy/server"
	"github.com/cnlh/easyProxy/utils"
	_ "github.com/cnlh/easyProxy/web/routers"
	"log"
)

var (
	TcpPort      = flag.Int("tcpport", 0, "客户端与服务端通信端口")
	httpPort     = flag.Int("httpport", 8024, "对外监听的端口")
	rpMode       = flag.String("mode", "webServer", "启动模式")
	tunnelTarget = flag.String("target", "10.1.50.203:80", "远程目标")
	VerifyKey    = flag.String("vkey", "", "验证密钥")
	u            = flag.String("u", "", "验证用户名(socks5和web)")
	p            = flag.String("p", "", "验证密码(socks5和web)")
	compress     = flag.String("compress", "", "数据压缩方式（snappy）")
	crypt        = flag.String("crypt", "false", "是否加密(true|false)")
	mux          = flag.String("mux", "false", "是否TCP多路复用(true|false)")
)

func main() {
	flag.Parse()
	server.VerifyKey = *VerifyKey
	log.Println("服务端启动，监听tcp服务端端口：", *TcpPort)
	cnf := server.ServerConfig{
		TcpPort:        *httpPort,
		Mode:           *rpMode,
		Target:         *tunnelTarget,
		VerifyKey:      *VerifyKey,
		U:              *u,
		P:              *p,
		Compress:       *compress,
		Start:          0,
		IsRun:          0,
		ClientStatus:   0,
		Crypt:          utils.GetBoolByStr(*crypt),
		Mux:            utils.GetBoolByStr(*mux),
		CompressEncode: 0,
		CompressDecode: 0,
	}
	if *TcpPort == 0 {
		p, err := beego.AppConfig.Int("tcpport")
		if err == nil && *rpMode == "webServer" {
			*TcpPort = p
		} else {
			*TcpPort = 8284
		}
	}
	cnf.CompressDecode, cnf.CompressEncode = utils.GetCompressType(cnf.Compress)
	server.StartNewServer(*TcpPort, &cnf)
}
