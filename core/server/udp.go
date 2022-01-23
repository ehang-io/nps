package server

import (
	"ehang.io/nps/core/handler"
	"ehang.io/nps/lib/logger"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"net"
)

type UdpServer struct {
	ServerAddr string `json:"server_addr" required:"true" placeholder:"0.0.0.0:8080 or :8080" zh_name:"监听地址"`
	gp         *ants.PoolWithFunc
	packetConn net.PacketConn
	handlers   map[string]handler.Handler
}

type udpPacket struct {
	n    int
	buf  []byte
	addr net.Addr
}

func (us *UdpServer) Init() error {
	us.handlers = make(map[string]handler.Handler, 0)
	if err := us.listen(); err != nil {
		return err
	}
	var err error
	us.gp, err = ants.NewPoolWithFunc(1000000, func(i interface{}) {
		p := i.(*udpPacket)
		defer bp.Put(p.buf)

		logger.Debug("accept a now packet", zap.String("remote addr", p.addr.String()))

	})
	return err
}

func (us *UdpServer) GetServerAddr() string {
	if us.packetConn == nil {
		return us.ServerAddr
	}
	return us.packetConn.LocalAddr().String()
}

func (us *UdpServer) GetName() string {
	return "udp"
}

func (us *UdpServer) GetZhName() string {
	return "udp服务"
}

func (us *UdpServer) listen() error {
	addr, err := net.ResolveUDPAddr("udp", us.ServerAddr)
	if err != nil {
		return err
	}
	us.packetConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	return nil
}

func (us *UdpServer) Serve() {
	for {
		buf := bp.Get()
		n, addr, err := us.packetConn.ReadFrom(buf)
		if err != nil {
			logger.Error("accept packet failed", zap.Error(err))
			break
		}
		err = us.gp.Invoke(udpPacket{n: n, buf: buf, addr: addr})
		if err != nil {
			logger.Error("Invoke error", zap.Error(err))
		}
	}
}
