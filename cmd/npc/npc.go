package main

import (
	"flag"
	"github.com/cnlh/nps/client"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"os"
	"strings"
	"time"
)

var (
	serverAddr   = flag.String("server", "", "Server addr (ip:port)")
	configPath   = flag.String("config", "", "Configuration file path")
	verifyKey    = flag.String("vkey", "", "Authentication key")
	logType      = flag.String("log", "stdout", "Log output mode（stdout|file）")
	connType     = flag.String("type", "tcp", "Connection type with the server（kcp|tcp）")
	proxyUrl     = flag.String("proxy", "", "proxy socks5 url(eg:socks5://111:222@127.0.0.1:9007)")
	logLevel     = flag.String("log_level", "7", "log level 0~7")
	registerTime = flag.Int("time", 2, "register time long /h")
)

func main() {
	flag.Parse()
	if len(os.Args) > 2 {
		switch os.Args[1] {
		case "status":
			path := strings.Replace(os.Args[2], "-config=", "", -1)
			client.GetTaskStatus(path)
		case "register":
			flag.CommandLine.Parse(os.Args[2:])
			client.RegisterLocalIp(*serverAddr, *verifyKey, *connType, *proxyUrl, *registerTime)
		}
	}
	daemon.InitDaemon("npc", common.GetRunPath(), common.GetTmpPath())
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	if *logType == "stdout" {
		logs.SetLogger(logs.AdapterConsole, `{"level":`+*logLevel+`,"color":true}`)
	} else {
		logs.SetLogger(logs.AdapterFile, `{"level":`+*logLevel+`,"filename":"npc_log.log"}`)
	}
	env := common.GetEnvMap()
	if *serverAddr == "" {
		*serverAddr, _ = env["NPS_SERVER_ADDR"]
	}
	if *verifyKey == "" {
		*verifyKey, _ = env["NPS_SERVER_VKEY"]
	}
	if *verifyKey != "" && *serverAddr != "" && *configPath == "" {
		for {
			client.NewRPClient(*serverAddr, *verifyKey, *connType, *proxyUrl).Start()
			logs.Info("It will be reconnected in five seconds")
			time.Sleep(time.Second * 5)
		}
	} else {
		if *configPath == "" {
			*configPath = "npc.conf"
		}
		client.StartFromFile(*configPath)
	}
}
