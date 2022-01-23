package handler

import (
	"ehang.io/nps/lib/enet"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHandleP2PPacket(t *testing.T) {

	h := P2PHandler{}
	rule := &testRule{}
	h.AddRule(rule)
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	assert.NoError(t, err)
	pc := enet.NewReaderPacketConn(nil, []byte("p2p  xxxx"), addr)

	assert.NoError(t, pc.SendPacket([]byte("p2p  xxxx"), nil))

	res, err := h.HandlePacketConn(pc)

	assert.NoError(t, err)
	assert.Equal(t, true, res)
	assert.Equal(t, true, rule.run)
}
