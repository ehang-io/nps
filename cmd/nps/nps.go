package main

import (
	"flag"
	"github.com/cnlh/easyProxy/client"
	"github.com/cnlh/easyProxy/utils"
	_ "github.com/cnlh/easyProxy/utils"
	"strings"
)

const VERSION = "v0.0.13"

var (
	serverAddr = flag.String("server", "", "服务器地址ip:端口")
	verifyKey  = flag.String("vkey", "", "验证密钥")
	logType    = flag.String("log", "stdout", "日志输出方式（stdout|file）")
)

func main() {
	flag.Parse()
	utils.InitDaemon("client")
	if *logType == "stdout" {
		utils.InitLogFile("client", true)
	} else {
		utils.InitLogFile("client", false)
	}
	stop := make(chan int)
	for _, v := range strings.Split(*verifyKey, ",") {
		utils.Println("客户端启动，连接：", *serverAddr, " 验证令牌：", v)
		go client.NewRPClient(*serverAddr, v).Start()
	}
	<-stop
}
