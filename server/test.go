package server

import (
	"github.com/astaxie/beego"
	"github.com/cnlh/nps/lib"
	"log"
	"strconv"
)

func TestServerConfig() {
	var postArr []int
	for _, v := range lib.GetCsvDb().Tasks {
		isInArr(&postArr, v.TcpPort, v.Remark)
	}
	p, err := beego.AppConfig.Int("httpport")
	if err != nil {
		log.Fatalln("Getting web management port error :", err)
	} else {
		isInArr(&postArr, p, "WebmManagement port")
	}
	if p := beego.AppConfig.String("httpProxyPort"); p != "" {
		if port, err := strconv.Atoi(p); err != nil {
			log.Fatalln("get http port error:", err)
		} else {
			isInArr(&postArr, port, "https port")
		}
	}
	if p := beego.AppConfig.String("httpsProxyPort"); p != "" {
		if port, err := strconv.Atoi(p); err != nil {
			log.Fatalln("get https port error", err)
		} else {
			if !lib.FileExists(beego.AppConfig.String("pemPath")) {
				log.Fatalf("ssl certFile %s is not exist", beego.AppConfig.String("pemPath"))
			}
			if !lib.FileExists(beego.AppConfig.String("ketPath")) {
				log.Fatalf("ssl keyFile %s is not exist", beego.AppConfig.String("pemPath"))
			}
			isInArr(&postArr, port, "http port")
		}
	}
}

func isInArr(arr *[]int, val int, remark string) {
	for _, v := range *arr {
		if v == val {
			log.Fatalf("the port %d is reused,remark: %s", val, remark)
		}
	}
	if !lib.TestTcpPort(val) {
		log.Fatalf("open the %d port error ,remark: %s", val, remark)
	}
	*arr = append(*arr, val)
	return
}
