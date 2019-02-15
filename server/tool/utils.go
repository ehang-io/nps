package tool

import (
	"github.com/cnlh/nps/lib/beego"
	"github.com/cnlh/nps/lib/common"
)

var ports []int

func init() {
	p := beego.AppConfig.String("allowPorts")
	ports = common.GetPorts(p)
}

func TestServerPort(p int, m string) (b bool) {
	if len(ports) != 0 {
		if !common.InIntArr(ports, p) {
			return false
		}
	}
	if m == "udpServer" {
		b = common.TestUdpPort(p)
	} else {
		b = common.TestTcpPort(p)
	}
	return
}
