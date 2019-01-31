package main

import (
	"flag"
	"github.com/cnlh/easyProxy/client"
	"log"
	"strings"
)

var (
	serverAddr = flag.String("server", "", "服务器地址ip:端口")
	verifyKey  = flag.String("vkey", "", "验证密钥")
)

func main() {
	log.SetFlags(log.Lshortfile)
	flag.Parse()
	stop := make(chan int)
	for _, v := range strings.Split(*verifyKey, ",") {
		log.Println("客户端启动，连接：", *serverAddr, " 验证令牌：", v)
		go client.NewRPClient(*serverAddr, v).Start()
	}
	<-stop
}
