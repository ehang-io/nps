package conn

import (
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/rate"
	"net"
)

type Link struct {
	Id            int    //id
	ConnType      string //连接类型
	Host          string //目标
	En            int    //加密
	De            int    //解密
	Crypt         bool   //加密
	Conn          *Conn
	Flow          *file.Flow
	UdpListener   *net.UDPConn
	Rate          *rate.Rate
	UdpRemoteAddr *net.UDPAddr
	Stop          chan bool
}

func NewLink(id int, connType string, host string, en, de int, crypt bool, c *Conn, flow *file.Flow, udpListener *net.UDPConn, rate *rate.Rate, UdpRemoteAddr *net.UDPAddr) *Link {
	return &Link{
		Id:            id,
		ConnType:      connType,
		Host:          host,
		En:            en,
		De:            de,
		Crypt:         crypt,
		Conn:          c,
		Flow:          flow,
		UdpListener:   udpListener,
		Rate:          rate,
		UdpRemoteAddr: UdpRemoteAddr,
	}
}
