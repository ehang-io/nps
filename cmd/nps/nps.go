package main

import (
	"flag"
	"github.com/astaxie/beego"
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/lib"
	_ "github.com/cnlh/nps/web/routers"
	"os"
)

const VERSION = "v0.0.13"

var (
	TcpPort      = flag.Int("tcpport", 0, "客户端与服务端通信端口")
	httpPort     = flag.Int("httpport", 8024, "对外监听的端口")
	rpMode       = flag.String("mode", "webServer", "启动模式")
	tunnelTarget = flag.String("target", "127.0.0.1:80", "远程目标")
	VerifyKey    = flag.String("vkey", "", "验证密钥")
	u            = flag.String("u", "", "验证用户名(socks5和web)")
	p            = flag.String("p", "", "验证密码(socks5和web)")
	compress     = flag.String("compress", "", "数据压缩方式（snappy）")
	crypt        = flag.String("crypt", "false", "是否加密(true|false)")
	logType      = flag.String("log", "stdout", "日志输出方式（stdout|file）")
)

func main() {
	flag.Parse()
	var test bool
	if len(os.Args) > 1 && os.Args[1] == "test" {
		test = true
	}
	lib.InitDaemon("nps")
	if *logType == "stdout" || test {
		lib.InitLogFile("nps", true)
	} else {
		lib.InitLogFile("nps", false)
	}
	task := &lib.Tunnel{
		TcpPort: *httpPort,
		Mode:    *rpMode,
		Target:  *tunnelTarget,
		Config: &lib.Config{
			U:        *u,
			P:        *p,
			Compress: *compress,
			Crypt:    lib.GetBoolByStr(*crypt),
		},
		Flow:         &lib.Flow{},
		UseClientCnf: false,
	}
	if *VerifyKey != "" {
		c := &lib.Client{
			Id:        0,
			VerifyKey: *VerifyKey,
			Addr:      "",
			Remark:    "",
			Status:    true,
			IsConnect: false,
			Cnf:       &lib.Config{},
			Flow:      &lib.Flow{},
		}
		c.Cnf.CompressDecode, c.Cnf.CompressEncode = lib.GetCompressType(c.Cnf.Compress)
		server.CsvDb.Clients[0] = c
		task.Client = c
	}
	if *TcpPort == 0 {
		p, err := beego.AppConfig.Int("tcpport")
		if err == nil && *rpMode == "webServer" {
			*TcpPort = p
		} else {
			*TcpPort = 8284
		}
	}
	lib.Println("服务端启动，监听tcp服务端端口：", *TcpPort)
	task.Config.CompressDecode, task.Config.CompressEncode = lib.GetCompressType(task.Config.Compress)
	if *rpMode != "webServer" {
		server.CsvDb.Tasks[0] = task
	}
	server.StartNewServer(*TcpPort, task, test)
}
