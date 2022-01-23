package action

import (
	"crypto/tls"
	"ehang.io/nps/lib/cert"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/pb"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"net"
)

type NpcAction struct {
	NpcId       string `json:"npc_id" required:"true" placeholder:"npc id" zh_name:"客户端"`
	BridgeAddr  string `json:"bridge_addr" placeholder:"127.0.0.1:8080" zh_name:"网桥地址"`
	UnixSocket  bool   `json:"unix_sock" placeholder:"" zh_name:"转发到unix socket"`
	networkTcp  pb.ConnType
	tlsConfig   *tls.Config
	connRequest *pb.ConnRequest
	DefaultAction
}

func (na *NpcAction) GetName() string {
	return "npc"
}

func (na *NpcAction) GetZhName() string {
	return "转发到客户端"
}

func (na *NpcAction) Init() error {
	if na.tlsConfig == nil {
		return errors.New("tls config is nil")
	}
	sn, err := cert.GetCertSnFromConfig(na.tlsConfig)
	if err != nil {
		return errors.Wrap(err, "get serial number")
	}
	na.connRequest = &pb.ConnRequest{Id: sn}
	na.networkTcp = pb.ConnType_tcp
	if na.UnixSocket {
		// just support unix
		na.networkTcp = pb.ConnType_unix
	}
	return nil
}

func (na *NpcAction) RunConnWithAddr(clientConn net.Conn, addr string) error {
	serverConn, err := na.GetServeConnWithAddr(addr)
	if err != nil {
		return err
	}
	na.startCopy(clientConn, serverConn)
	return nil
}

func (na *NpcAction) CanServe() bool {
	return true
}

func (na *NpcAction) GetServeConnWithAddr(addr string) (net.Conn, error) {
	return dialBridge(na, na.networkTcp, addr)
}

func (na *NpcAction) RunPacketConn(pc net.PacketConn) error {
	serverPacketConn, err := dialBridge(na, pb.ConnType_udp, "")
	if err != nil {
		return err
	}
	return na.startCopyPacketConn(pc, enet.NewTcpPacketConn(serverPacketConn))
}

func dialBridge(npc *NpcAction, connType pb.ConnType, addr string) (net.Conn, error) {
	tlsConn, err := tls.Dial("tcp", npc.BridgeAddr, npc.tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "dial bridge tls")
	}
	cr := proto.Clone(npc.connRequest).(*pb.ConnRequest)
	cr.ConnType = &pb.ConnRequest_AppInfo{AppInfo: &pb.AppInfo{ConnType: connType, AppAddr: addr, NpcId: npc.NpcId}}
	if _, err = pb.WriteMessage(tlsConn, cr); err != nil {
		return nil, errors.Wrap(err, "write enet request")
	}
	return tlsConn, err
}
