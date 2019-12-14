package main

import (
	"flag"
	"github.com/cnlh/nps/lib/install"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/version"
	"github.com/cnlh/nps/server"
	"github.com/cnlh/nps/server/connection"
	"github.com/cnlh/nps/server/tool"

	"github.com/cnlh/nps/web/routers"
	"github.com/kardianos/service"
)

var (
	level   string
	logType = flag.String("log", "stdout", "Log output mode（stdout|file）")
)

func main() {
	flag.Parse()
	// init log
	if err := beego.LoadAppConfig("ini", filepath.Join(common.GetRunPath(), "conf", "nps.conf")); err != nil {
		log.Fatalln("load config file error", err.Error())
	}
	if level = beego.AppConfig.String("log_level"); level == "" {
		level = "7"
	}
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	logPath := beego.AppConfig.String("log_path")
	if logPath == "" {
		logPath = common.GetLogPath()
	}
	if common.IsWindows() {
		logPath = strings.Replace(logPath, "\\", "\\\\", -1)
	}
	logs.SetLogger(logs.AdapterFile, `{"level":`+level+`,"filename":"`+logPath+`","daily":false,"maxlines":100000,"color":true}`)
	// init service
	options := make(service.KeyValue)
	options["Restart"] = "on-success"
	options["SuccessExitStatus"] = "1 2 8 SIGKILL"
	svcConfig := &service.Config{
		Name:        "Nps",
		DisplayName: "nps内网穿透代理服务器",
		Description: "一款轻量级、功能强大的内网穿透代理服务器。支持tcp、udp流量转发，支持内网http代理、内网socks5代理，同时支持snappy压缩、站点保护、加密传输、多路复用、header修改等。支持web图形化管理，集成多用户模式。",
		Option:      options,
	}
	if !common.IsWindows() {
		svcConfig.Dependencies = []string{
			"Requires=network.target",
			"After=network-online.target syslog.target"}
	}
	prg := &nps{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logs.Error(err)
		return
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "debug":
			logs.SetLogger(logs.AdapterConsole, `{"level":`+level+`,"color":true}`)
		case "reload":
			daemon.InitDaemon("nps", common.GetRunPath(), common.GetTmpPath())
			return
		case "install":
			// uninstall before
			service.Control(s, "uninstall")

			binPath := install.InstallNps()
			svcConfig.Executable = binPath
			s, err := service.New(prg, svcConfig)
			if err != nil {
				logs.Error(err)
				return
			}
			err = service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n", service.ControlAction, err.Error())
			}
			return
		case "start", "restart", "stop", "uninstall":
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n", service.ControlAction, err.Error())
			}
			return
		case "update":
			install.Update()
			return
		default:
			logs.Error("command is not support")
			return
		}
	}
	s.Run()
}

type nps struct {
	exit chan struct{}
}

func (p *nps) Start(s service.Service) error {
	p.run()
	return nil
}
func (p *nps) Stop(s service.Service) error {
	close(p.exit)
	return nil
}

func (p *nps) run() error {
	routers.Init()
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
	go server.StartNewServer(bridgePort, task, beego.AppConfig.String("bridge_type"))
	select {
	case <-p.exit:
		logs.Warning("stop...")
	}
	return nil
}
