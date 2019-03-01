package tool

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
)

var ports []int

func init() {
	p := beego.AppConfig.String("allowPorts")
	ports = common.GetPorts(p)
}

func TestServerPort(p int, m string) (b bool) {
	if p > 65535 || p < 0 {
		return false
	}
	if len(ports) != 0 {
		if !common.InIntArr(ports, p) {
			return false
		}
	}
	if m == "udp" {
		b = common.TestUdpPort(p)
	} else {
		b = common.TestTcpPort(p)
	}
	return
}
