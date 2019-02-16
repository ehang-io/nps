package main

import (
	"flag"
	"github.com/cnlh/nps/client"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/lg"
	"os"
	"strings"
	"time"
)

const VERSION = "v0.0.15"

var (
	serverAddr   = flag.String("server", "", "Server addr (ip:port)")
	configPath   = flag.String("config", "npc.conf", "Configuration file path")
	verifyKey    = flag.String("vkey", "", "Authentication key")
	logType      = flag.String("log", "stdout", "Log output mode（stdout|file）")
	connType     = flag.String("type", "tcp", "Connection type with the server（kcp|tcp）")
	proxyUrl     = flag.String("proxy", "", "proxy socks5 url(eg:socks5://111:222@127.0.0.1:9007)")
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
	if *logType == "stdout" {
		lg.InitLogFile("npc", true, common.GetLogPath())
	} else {
		lg.InitLogFile("npc", false, common.GetLogPath())
	}
	if *verifyKey != "" && *serverAddr != "" {
		for {
			client.NewRPClient(*serverAddr, *verifyKey, *connType, *proxyUrl).Start()
			lg.Println("It will be reconnected in five seconds")
			time.Sleep(time.Second * 5)
		}
	} else {
		client.StartFromFile(*configPath)
	}
}
