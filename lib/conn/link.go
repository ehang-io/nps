package conn

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/pool"
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
	MsgCh         chan []byte
	MsgConn       *Conn
	StatusCh      chan bool
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
		MsgCh:         make(chan []byte),
		StatusCh:      make(chan bool),
	}
}

func (s *Link) Run(flow bool) {
	go func() {
		for {
			select {
			case content := <-s.MsgCh:
				if len(content) == len(common.IO_EOF) && string(content) == common.IO_EOF {
					if s.Conn != nil {
						s.Conn.Close()
					}
					return
				} else {
					if s.Conn == nil {
						return
					}
					if s.UdpListener != nil && s.UdpRemoteAddr != nil {
						s.UdpListener.WriteToUDP(content, s.UdpRemoteAddr)
					} else {
						s.Conn.Write(content)
					}
					if flow {
						s.Flow.Add(0, len(content))
					}
					if s.ConnType == common.CONN_UDP {
						return
					}
					s.MsgConn.WriteWriteSuccess(s.Id)
					pool.PutBufPoolCopy(content)
				}
			}
		}
	}()
}
