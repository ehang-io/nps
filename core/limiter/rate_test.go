package limiter

import (
	"bytes"
	"ehang.io/nps/lib/enet"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	cl := RateLimiter{
		RateLimit: 100,
	}
	assert.NoError(t, cl.Init())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	nowBytes := 0
	close := make(chan struct{})
	go func() {
		buf := make([]byte, 10)
		c, err := ln.Accept()
		assert.NoError(t, err)
		c, err = cl.DoLimit(enet.NewReaderConn(c))
		go func() {
			<-time.After(time.Second * 2)
			if nowBytes > 500 {
				t.Fail()
			}
			close <- struct{}{}
		}()
		for {
			n, err := c.Read(buf)
			nowBytes += n
			assert.NoError(t, err)
		}
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)
	for i := 11; i > 0; i-- {
		_, err := c.Write(bytes.Repeat([]byte{0}, 10000))
		assert.NoError(t, err)
	}
	<-close
}
