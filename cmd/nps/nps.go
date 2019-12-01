package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/install"
	"github.com/cnlh/nps/lib/version"
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/server/connection"
	"github.com/cnlh/nps/server/test"
	"github.com/cnlh/nps/server/tool"
	_ "github.com/cnlh/nps/web/routers"
)

var (
	level   string
	logType = flag.String("log", "stdout", "Log output mode（stdout|file）")
)

func main() {
	flag.Parse()
	beego.LoadAppConfig("ini", filepath.Join(common.GetRunPath(), "conf", "nps.conf"))
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "test":
			test.TestServerConfig()
			log.Println("test ok, no error")
			return
		case "start", "restart", "stop", "status", "reload":
			daemon.InitDaemon("nps", common.GetRunPath(), common.GetTmpPath())
		case "install":
			install.InstallNps()
			return
		}
	}
	if level = beego.AppConfig.String("log_level"); level == "" {
		level = "7"
	}
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	if *logType == "stdout" {
		logs.SetLogger(logs.AdapterConsole, `{"level":`+level+`,"color":true}`)
	} else {
		logs.SetLogger(logs.AdapterFile, `{"level":`+level+`,"filename":"`+beego.AppConfig.String("log_path")+`","daily":false,"maxlines":100000,"color":true}`)
	}
	task := &file.Tunnel{
		Mode: "webServer",
	}
	bridgePort, err := beego.AppConfig.Int("bridge_port")
	if err != nil {
		logs.Error("Getting bridge_port error", err)
		os.Exit(0)
	}
	logs.Info("the version of server is %s ,allow client core version to be %s", version.VERSION, version.GetVersion())
	connection.InitConnectionService()
	crypt.InitTls(filepath.Join(common.GetRunPath(), "conf", "server.pem"), filepath.Join(common.GetRunPath(), "conf", "server.key"))
	tool.InitAllowPort()
	tool.StartSystemInfo()
	server.StartNewServer(bridgePort, task, beego.AppConfig.String("bridge_type"))
}
