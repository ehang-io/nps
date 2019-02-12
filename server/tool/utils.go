package tool

import (
	"github.com/cnlh/nps/lib/beego"
	"github.com/cnlh/nps/lib/common"
	"strconv"
	"strings"
)

var ports []int

func init() {
	p := beego.AppConfig.String("allowPorts")
	arr := strings.Split(p, ",")
	for _, v := range arr {
		fw := strings.Split(v, "-")
		if len(fw) == 2 {
			if isPort(fw[0]) && isPort(fw[1]) {
				start, _ := strconv.Atoi(fw[0])
				end, _ := strconv.Atoi(fw[1])
				for i := start; i <= end; i++ {
					ports = append(ports, i)
				}
			} else {
				continue
			}
		} else if isPort(v) {
			p, _ := strconv.Atoi(v)
			ports = append(ports, p)
		}
	}
}
func isPort(p string) bool {
	pi, err := strconv.Atoi(p)
	if err != nil {
		return false
	}
	if pi > 65536 || pi < 1 {
		return false
	}
	return true
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
