package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/pb"
	"github.com/pkg/errors"
)

type PbAppProcessor struct {
	DefaultProcess
}

func (pp *PbAppProcessor) GetName() string {
	return "pb_app"
}

func (pp *PbAppProcessor) ProcessConn(c enet.Conn) (bool, error) {
	m := &pb.ClientRequest{}
	n, err := pb.ReadMessage(c, m)
	if err != nil {
		return false, nil
	}
	if _, ok := m.ConnType.(*pb.ClientRequest_AppInfo); !ok {
		return false, nil
	}
	if err := c.Reset(n + 4); err != nil {
		return true, errors.Wrap(err, "reset connection data")
	}
	switch m.GetAppInfo().GetConnType() {
	case pb.ConnType_udp:
		return true, pp.RunUdp(c)
	case pb.ConnType_tcp:
		return true, pp.ac.RunConnWithAddr(c, m.GetAppInfo().GetAppAddr())
	case pb.ConnType_unix:
		ac := &action.LocalAction{TargetAddr: []string{m.GetAppInfo().GetAppAddr()}, UnixSocket: true}
		_ = ac.Init()
		return true, ac.RunConn(c)
	}
	return true, errors.Errorf("can not support the conn type(%d)", m.GetAppInfo().GetConnType())
}

func (pp *PbAppProcessor) RunUdp(c enet.Conn) error {
	return pp.ac.RunPacketConn(enet.NewTcpPacketConn(c))
}
