package common

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"net/http"
	_ "net/http/pprof"
)

func InitPProfFromFile() {
	ip := beego.AppConfig.String("pprof_ip")
	p := beego.AppConfig.String("pprof_port")
	if len(ip) > 0 && len(p) > 0 && IsPort(p) {
		runPProf(ip + ":" + p)
	}
}

func InitPProfFromArg(arg string) {
	if len(arg) > 0 {
		runPProf(arg)
	}
}

func runPProf(ipPort string) {
	go func() {
		_ = http.ListenAndServe(ipPort, nil)
	}()
	logs.Info("PProf debug listen on", ipPort)
}
