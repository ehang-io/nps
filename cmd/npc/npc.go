package main

import (
	"flag"
	"github.com/cnlh/nps/client"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/lib/common"
	"strings"
)

const VERSION = "v0.0.13"

var (
	serverAddr = flag.String("server", "", "服务器地址ip:端口")
	verifyKey  = flag.String("vkey", "", "验证密钥")
	logType    = flag.String("log", "stdout", "日志输出方式（stdout|file）")
	connType   = flag.String("type", "tcp", "与服务端建立连接方式（kcp|tcp）")
)

func main() {
	flag.Parse()
	daemon.InitDaemon("npc", common.GetRunPath(), common.GetPidPath())
	if *logType == "stdout" {
		lg.InitLogFile("npc", true, common.GetLogPath())
	} else {
		lg.InitLogFile("npc", false, common.GetLogPath())
	}
	stop := make(chan int)
	for _, v := range strings.Split(*verifyKey, ",") {
		lg.Println("客户端启动，连接：", *serverAddr, " 验证令牌：", v)
		go client.NewRPClient(*serverAddr, v, *connType).Start()
	}
	<-stop
}
