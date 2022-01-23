package client

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/rule"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"go.uber.org/zap"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	controlLn    net.Listener
	dataLn       net.Listener
	lastPongTime time.Time
	mux          *http.ServeMux
	ticker       *time.Ticker
	closeCh      chan struct{}
	closed       int32
	wg           sync.WaitGroup
}

func NewClient(controlLn, dataLn net.Listener) *Client {
	return &Client{
		controlLn: controlLn,
		dataLn:    dataLn,
		mux:       &http.ServeMux{},
		ticker:    time.NewTicker(time.Second * 5),
		closeCh:   make(chan struct{}, 0),
	}
}

func (c *Client) ping(writer http.ResponseWriter, request *http.Request) {
	c.lastPongTime = time.Now()
	_, err := io.WriteString(writer, "pong")
	if err != nil {
		logger.Warn("write pong error", zap.Error(err))
		return
	}
	logger.Debug("write pong success")
}

func (c *Client) Run() {
	c.mux.HandleFunc("/ping", c.ping)
	c.wg.Add(3)
	go c.handleControlConn()
	go c.handleDataConn()
	go c.checkPing()
	c.wg.Wait()
}

func (c *Client) HasPong() bool {
	return time.Now().Sub(c.lastPongTime).Seconds() < 10
}

func (c *Client) checkPing() {
	for {
		select {
		case <-c.ticker.C:
			if !c.lastPongTime.IsZero() && time.Now().Sub(c.lastPongTime).Seconds() > 15 && c.controlLn != nil {
				logger.Debug("close connection", zap.Time("lastPongTime", c.lastPongTime), zap.Time("now", time.Now()))
				_ = c.controlLn.Close()
			}
		case <-c.closeCh:
			c.wg.Done()
			return
		}
	}
}

func (c *Client) handleDataConn() {
	h := &handler.DefaultHandler{}
	ac := &action.LocalAction{}
	err := ac.Init()
	if err != nil {
		logger.Warn("init action failed", zap.Error(err))
		return
	}
	appPr := &process.PbAppProcessor{}
	_ = appPr.Init(ac)
	h.AddRule(&rule.Rule{Handler: h, Process: appPr, Action: ac})

	pingPr := &process.PbPingProcessor{}
	_ = appPr.Init(ac)
	h.AddRule(&rule.Rule{Handler: h, Process: pingPr, Action: ac})

	var conn net.Conn
	for {
		conn, err = c.dataLn.Accept()
		if err != nil {
			logger.Error("accept connection failed", zap.Error(err))
			break
		}
		go func(conn net.Conn) {
			_, err = h.HandleConn(nil, enet.NewReaderConn(conn))
			if err != nil {
				logger.Warn("process failed", zap.Error(err))
				return
			}
		}(conn)
	}
	c.wg.Done()
	c.Close()
}

func (c *Client) handleControlConn() {
	err := http.Serve(c.controlLn, c.mux)
	if err != nil {
		logger.Error("http error", zap.Error(err))
	}
	c.wg.Done()
	c.Close()
}

func (c *Client) Close() {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		c.closeCh <- struct{}{}
		c.ticker.Stop()
		_ = c.controlLn.Close()
		_ = c.dataLn.Close()
	}
}
