package limiter

import (
	"bytes"
	"ehang.io/nps/lib/enet"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestFlowLimiter(t *testing.T) {
	cl := FlowLimiter{
		OutLimit: 100,
		InLimit:  100,
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
		for {
			n, err := c.Read(buf)
			nowBytes += n
			if nowBytes > 100 {
				assert.Error(t, err)
				nowBytes = 0
				for i := 11; i > 0; i-- {
					n, err = c.Write(bytes.Repeat([]byte{0}, 10))
					nowBytes += n
					if nowBytes > 100 {
						assert.Error(t, err)
						close <- struct{}{}
					} else {
						assert.NoError(t, err)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		}
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	assert.NoError(t, err)
	for i := 11; i > 0; i-- {
		_, err := c.Write(bytes.Repeat([]byte{0}, 10))
		assert.NoError(t, err)
	}
	buf := make([]byte, 10)
	for i := 11; i > 0; i-- {
		_, err := c.Read(buf)
		assert.NoError(t, err)
	}
	<-close
}
