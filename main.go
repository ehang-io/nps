package main

import (
	"flag"
	"fmt"
	"log"
)

var (
	configPath = flag.String("config", "config.json", "配置文件路径")
	tcpPort    = flag.Int("tcpport", 8284, "Socket连接或者监听的端口")
	httpPort   = flag.Int("httpport", 8024, "当mode为server时为服务端监听端口，当为mode为client时为转发至本地客户端的端口")
	rpMode     = flag.String("mode", "client", "启动模式，可选为client、server")
	verifyKey  = flag.String("vkey", "", "验证密钥")
	config     Config
	err        error
)

func main() {
	flag.Parse()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if *rpMode == "client" {
		JsonParse := NewJsonStruct()
		config, err = JsonParse.Load(*configPath)
		if err != nil {
			log.Fatalln(err)
		}
		*verifyKey = config.Server.Vkey
		log.Println("客户端启动，连接：", config.Server.Ip, "， 端口：", config.Server.Tcp)
		cli := NewRPClient(fmt.Sprintf("%s:%d", config.Server.Ip, config.Server.Tcp), config.Server.Num)
		cli.Start()
	} else if *rpMode == "server" {
		if *verifyKey == "" {
			log.Fatalln("必须输入一个验证的key")
		}
		if *tcpPort <= 0 || *tcpPort >= 65536 {
			log.Fatalln("请输入正确的tcp端口。")
		}
		if *httpPort <= 0 || *httpPort >= 65536 {
			log.Fatalln("请输入正确的http端口。")
		}
		log.Println("服务端启动，监听tcp服务端端口：", *tcpPort, "， http服务端端口：", *httpPort)
		svr := NewRPServer(*tcpPort, *httpPort)
		if err := svr.Start(); err != nil {
			log.Fatalln(err)
		}
		defer svr.Close()
	}
}
