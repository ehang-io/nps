package main

import (
	"flag"
	"github.com/cnlh/nps/lib/beego"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/install"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/server"
	_ "github.com/cnlh/nps/web/routers"
	"log"
	"os"
	"path/filepath"
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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "test":
			server.TestServerConfig()
			log.Println("test ok, no error")
			return
		case "start", "restart", "stop", "status":
			daemon.InitDaemon("nps", common.GetRunPath(), common.GetPidPath())
		case "install":
			install.InstallNps()
			return
		}
	}
	if *logType == "stdout" {
		lg.InitLogFile("nps", true, common.GetLogPath())
	} else {
		lg.InitLogFile("nps", false, common.GetLogPath())
	}
	task := &file.Tunnel{
		TcpPort: *httpPort,
		Mode:    *rpMode,
		Target:  *tunnelTarget,
		Config: &file.Config{
			U:        *u,
			P:        *p,
			Compress: *compress,
			Crypt:    common.GetBoolByStr(*crypt),
		},
		Flow:         &file.Flow{},
		UseClientCnf: false,
	}
	if *VerifyKey != "" {
		c := &file.Client{
			Id:        0,
			VerifyKey: *VerifyKey,
			Addr:      "",
			Remark:    "",
			Status:    true,
			IsConnect: false,
			Cnf:       &file.Config{},
			Flow:      &file.Flow{},
		}
		c.Cnf.CompressDecode, c.Cnf.CompressEncode = common.GetCompressType(c.Cnf.Compress)
		file.GetCsvDb().Clients[0] = c
		task.Client = c
	}
	if *TcpPort == 0 {
		p, err := beego.AppConfig.Int("bridgePort")
		if err == nil && *rpMode == "webServer" {
			*TcpPort = p
		} else {
			*TcpPort = 8284
		}
	}
	lg.Printf("服务端启动，监听%s服务端口：%d", beego.AppConfig.String("bridgeType"), *TcpPort)
	task.Config.CompressDecode, task.Config.CompressEncode = common.GetCompressType(task.Config.Compress)
	if *rpMode != "webServer" {
		file.GetCsvDb().Tasks[0] = task
	}
	beego.LoadAppConfig("ini", filepath.Join(common.GetRunPath(), "conf", "app.conf"))
	server.StartNewServer(*TcpPort, task, beego.AppConfig.String("bridgeType"))
}
