package main

import (
	"flag"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/install"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/server/test"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	_ "github.com/cnlh/nps/web/routers"
	"log"
	"os"
	"path/filepath"
)

const VERSION = "v0.0.15"

var (
	logType = flag.String("log", "stdout", "Log output mode（stdout|file）")
)

func main() {
	log.SetFlags(log.Lshortfile)
	flag.Parse()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "test":
			test.TestServerConfig()
			log.Println("test ok, no error")
			return
		case "start", "restart", "stop", "status":
			daemon.InitDaemon("nps", common.GetRunPath(), common.GetTmpPath())
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
		Mode: "webServer",
	}
	bridgePort, err := beego.AppConfig.Int("bridgePort")
	if err != nil {
		lg.Fatalln("Getting bridgePort error", err)
	}
	beego.LoadAppConfig("ini", filepath.Join(common.GetRunPath(), "conf", "app.conf"))
	server.StartNewServer(bridgePort, task, beego.AppConfig.String("bridgeType"))
}
