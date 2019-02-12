package main

import (
	"flag"
	"github.com/cnlh/nps/client"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/lg"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const VERSION = "v0.0.15"

var (
	serverAddr = flag.String("server", "", "Server addr (ip:port)")
	configPath = flag.String("config", "npc.conf", "Configuration file path")
	verifyKey  = flag.String("vkey", "", "Authentication key")
	logType    = flag.String("log", "stdout", "Log output mode（stdout|file）")
	connType   = flag.String("type", "tcp", "Connection type with the server（kcp|tcp）")
)

func main() {
	flag.Parse()
	if len(os.Args) > 2 {
		switch os.Args[1] {
		case "status":
			path := strings.Replace(os.Args[2], "-config=", "", -1)
			cnf, err := config.NewConfig(path)
			if err != nil {
				log.Fatalln(err)
			}
			c, err := client.NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG)
			if err != nil {
				log.Fatalln(err)
			}
			if _, err := c.Write([]byte(common.WORK_STATUS)); err != nil {
				log.Fatalln(err)
			}
			if f, err := common.ReadAllFromFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt")); err != nil {
				log.Fatalln(err)
			} else if _, err := c.Write([]byte(string(f))); err != nil {
				log.Fatalln(err)
			}
			if l, err := c.GetLen(); err != nil {
				log.Fatalln(err)
			} else if b, err := c.ReadLen(l); err != nil {
				lg.Fatalln(err)
			} else {
				arr := strings.Split(string(b), common.CONN_DATA_SEQ)
				for _, v := range cnf.Hosts {
					if common.InArr(arr, v.Remark) {
						log.Println(v.Remark, "ok")
					} else {
						log.Println(v.Remark, "not running")
					}
				}
				for _, v := range cnf.Tasks {
					if common.InArr(arr, v.Remark) {
						log.Println(v.Remark, "ok")
					} else {
						log.Println(v.Remark, "not running")
					}
				}
			}
			return
		}
	}
	daemon.InitDaemon("npc", common.GetRunPath(), common.GetTmpPath())
	if *logType == "stdout" {
		lg.InitLogFile("npc", true, common.GetLogPath())
	} else {
		lg.InitLogFile("npc", false, common.GetLogPath())
	}
	if *verifyKey != "" && *serverAddr != "" {
		client.NewRPClient(*serverAddr, *verifyKey, *connType).Start()
	} else {
		client.StartFromFile(*configPath)
	}
}
