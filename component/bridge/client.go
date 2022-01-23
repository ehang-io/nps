package bridge

import (
	"context"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/lib/pb"
	"ehang.io/nps/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net"
	"net/http"
	"time"
)

type client struct {
	*manager
	tunnelId     string
	clientId     string
	control      transport.Conn
	data         transport.Conn
	httpClient   *http.Client
	pingErrTimes int
}

func NewClient(tunnelId string, clientId string, mg *manager) *client {
	c := &client{tunnelId: tunnelId, clientId: clientId, manager: mg}
	c.httpClient = &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return c.NewControlConn()
	}}}
	go c.doPing()
	return c
}

func (c *client) SetTunnel(conn transport.Conn, control bool) {
	if control {
		c.control = conn
	} else {
		c.data = conn
	}
}

func (c *client) NewDataConn() (net.Conn, error) {
	if c.data == nil {
		return nil, errors.New("the data tunnel is not exist")
	}
	return c.data.Open()
}

func (c *client) NewControlConn() (net.Conn, error) {
	if c.control == nil {
		return nil, errors.New("the data tunnel is not exist")
	}
	return c.control.Open()
}

func (c *client) doPing() {
	for range time.NewTicker(time.Second * 5).C {
		if err := c.ping(); err != nil {
			c.pingErrTimes++
			logger.Error("do ping error", zap.Error(err))
		} else {
			logger.Debug("do ping success", zap.String("client id", c.clientId), zap.String("tunnel id", c.tunnelId))
			c.pingErrTimes = 0
		}
		if c.pingErrTimes > 3 {
			logger.Error("ping failed, close")
			c.close()
			return
		}
	}
}

func (c *client) close() {
	if c.data != nil {
		_ = c.data.Close()
	}
	if c.control != nil {
		_ = c.control.Close()
	}
	_ = c.RemoveClient(c)
}

func (c *client) ping() error {
	conn, err := c.NewDataConn()
	if err != nil {
		return errors.Wrap(err, "data ping")
	}
	_, err = pb.WriteMessage(conn, &pb.ClientRequest{ConnType: &pb.ClientRequest_Ping{Ping: &pb.Ping{Now: time.Now().String()}}})
	if err != nil {
		return errors.Wrap(err, "data ping")
	}
	_, err = pb.ReadMessage(conn, &pb.Ping{})
	if err != nil {
		return errors.Wrap(err, "data ping")
	}
	resp, err := c.httpClient.Get("http://nps.ehang.io/ping")
	if err != nil {
		return errors.Wrap(err, "control ping")
	}
	_ = resp.Body.Close()
	return nil
}
