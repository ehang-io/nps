package handler

import (
	"context"
	"ehang.io/nps/lib/enet"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHandleRedisConn(t *testing.T) {
	h := RedisHandler{}
	rule := &testRule{}
	h.AddRule(rule)

	finish := make(chan struct{}, 0)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		conn, err := ln.Accept()
		assert.NoError(t, err)
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		assert.NoError(t, err)
		res, err := h.HandleConn(buf[:n], enet.NewReaderConn(conn))
		assert.NoError(t, err)
		assert.Equal(t, true, res)
		assert.Equal(t, true, rule.run)
		finish <- struct{}{}
	}()

	go func() {
		rdb := redis.NewClient(&redis.Options{
			Addr: ln.Addr().String(),
		})
		rdb.Ping(context.Background())
	}()
	<-finish
}
