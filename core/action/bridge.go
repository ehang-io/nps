package action

import (
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/pool"
	"net"
)

var bridgeListener = enet.NewListener()
var bridgePacketConn enet.PacketConn
var packetBp = pool.NewBufferPool(1500)

func GetBridgeListener() net.Listener {
	return bridgeListener
}

func GetBridgePacketConn() net.PacketConn {
	return bridgePacketConn
}

type BridgeAction struct {
	DefaultAction
	WritePacketConn net.PacketConn `json:"-"`
}

func (ba *BridgeAction) GetName() string {
	return "bridge"
}

func (ba *BridgeAction) GetZhName() string {
	return "转发到网桥"
}

func (ba *BridgeAction) Init() error {
	bridgePacketConn = enet.NewReaderPacketConn(ba.WritePacketConn, nil, ba.WritePacketConn.LocalAddr())
	return nil
}

func (ba *BridgeAction) RunConn(clientConn net.Conn) error {
	return bridgeListener.SendConn(clientConn)
}

func (ba *BridgeAction) RunConnWithAddr(clientConn net.Conn, addr string) error {
	return bridgeListener.SendConn(clientConn)
}

func (ba *BridgeAction) RunPacketConn(pc net.PacketConn) error {
	b := packetBp.Get()
	defer packetBp.Put(b)
	for {
		n, addr, err := pc.ReadFrom(b)
		if err != nil {
			break
		}
		err = bridgePacketConn.SendPacket(b[:n], addr)
		if err != nil {
			break
		}
	}
	return nil
}
