package bridge

import (
	"ehang.io/nps/lib/lb"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/transport"
	"go.uber.org/zap"
	"net"
	"sync"
)

type manager struct {
	clients  map[string]*client
	clientLb *lb.LoadBalancer
	sync.Mutex
}

func NewManager() *manager {
	return &manager{
		clients:  make(map[string]*client),
		clientLb: lb.NewLoadBalancer(),
	}
}

func (m *manager) SetClient(clientId string, tunnelId string, isControl bool, conn transport.Conn) error {
	m.Lock()
	defer m.Unlock()
	client, ok := m.clients[tunnelId]
	if !ok {
		client = NewClient(tunnelId, clientId, m)
		err := m.clientLb.SetClient(clientId, client)
		if err != nil {
			logger.Error("set client error", zap.Error(err), zap.String("clientId", clientId), zap.String("tunnelId", tunnelId))
			return err
		}
		m.clients[tunnelId] = client
	}
	client.SetTunnel(conn, isControl)
	return nil
}

func (m *manager) GetDataConn(clientId string) (net.Conn, error) {
	c, err := m.clientLb.GetClient(clientId)
	if err != nil {
		return nil, err
	}
	return c.(*client).NewDataConn()
}

func (m *manager) RemoveClient(client *client) error {
	m.Lock()
	defer m.Unlock()
	err := m.clientLb.RemoveClient(client.clientId, client)
	if err != nil {
		logger.Error("remove client error", zap.Error(err), zap.String("clientId", client.clientId), zap.String("tunnelId", client.tunnelId))
		return err
	}
	delete(m.clients, client.tunnelId)
	return nil
}
