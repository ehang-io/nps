package action

import (
	"ehang.io/nps/lib/lb"
	"net"
)

type LocalAction struct {
	DefaultAction
	TargetAddr []string `json:"target_addr" placeholder:"1.1.1.1:80\n1.1.1.2:80" zh_name:"目标地址"`
	UnixSocket bool     `json:"unix_sock" placeholder:"" zh_name:"转发到unix socket"`
	networkTcp string
	localLb    lb.Algo
}

func (la *LocalAction) GetName() string {
	return "local"
}

func (la *LocalAction) GetZhName() string {
	return "转发到本地"
}

func (la *LocalAction) Init() error {
	la.localLb = lb.GetLbAlgo("roundRobin")
	for _, v := range la.TargetAddr {
		_ = la.localLb.Append(v)
	}
	la.networkTcp = "tcp"
	if la.UnixSocket {
		// just support unix
		la.networkTcp = "unix"
	}
	return nil
}

func (la *LocalAction) RunConn(clientConn net.Conn) error {
	serverConn, err := la.GetServerConn()
	if err != nil {
		return err
	}
	la.startCopy(clientConn, serverConn)
	return nil
}

func (la *LocalAction) RunConnWithAddr(clientConn net.Conn, addr string) error {
	serverConn, err := la.GetServeConnWithAddr(addr)
	if err != nil {
		return err
	}
	la.startCopy(clientConn, serverConn)
	return nil
}

func (la *LocalAction) CanServe() bool {
	return true
}

func (la *LocalAction) GetServerConn() (net.Conn, error) {
	addr, err := la.localLb.Next()
	if err != nil {
		return nil, err
	}
	return la.GetServeConnWithAddr(addr.(string))
}

func (la *LocalAction) GetServeConnWithAddr(addr string) (net.Conn, error) {
	return net.Dial(la.networkTcp, addr)
}

func (la *LocalAction) RunPacketConn(pc net.PacketConn) error {
	localPacketConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	return la.startCopyPacketConn(pc, localPacketConn)
}
