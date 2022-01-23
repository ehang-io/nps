package process

import (
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/pb"
	"time"
)

type PbPingProcessor struct {
	DefaultProcess
}

func (pp *PbPingProcessor) GetName() string {
	return "pb_ping"
}

func (pp *PbPingProcessor) ProcessConn(c enet.Conn) (bool, error) {
	m := &pb.ClientRequest{}
	_, err := pb.ReadMessage(c, m)
	if err != nil {
		return false, nil
	}
	if _, ok := m.ConnType.(*pb.ClientRequest_Ping); !ok {
		return false, nil
	}
	m.GetPing().Now = time.Now().String()
	_, err = pb.WriteMessage(c, m)
	return true, err
}
