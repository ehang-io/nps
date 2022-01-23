package handler

import (
	"ehang.io/nps/lib/enet"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

type testRule struct {
	run bool
}

func (t *testRule) RunConn(c enet.Conn) (bool, error) {
	t.run = true
	return true, nil
}

func (t *testRule) RunPacketConn(_ enet.PacketConn) (bool, error) {
	t.run = true
	return true, nil
}

func TestHandleDnsPacket(t *testing.T) {
	lPacketConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	h := DnsHandler{}
	rule := &testRule{}
	h.AddRule(rule)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.google.com"), dns.TypeA)
	m.RecursionDesired = true

	b, err := m.Pack()
	assert.NoError(t, err)
	pc := enet.NewReaderPacketConn(nil, b, lPacketConn.LocalAddr())

	assert.NoError(t, pc.SendPacket(b, nil))
	res, err := h.HandlePacketConn(pc)

	assert.NoError(t, err)
	assert.Equal(t, true, res)
	assert.Equal(t, true, rule.run)
}
