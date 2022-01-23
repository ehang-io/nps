package limiter

import (
	"ehang.io/nps/lib/enet"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestConnNumLimiter(t *testing.T) {
	cl := ConnNumLimiter{MaxConnNum: 5}
	assert.NoError(t, cl.Init())
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	nowNum := 0
	close := make(chan struct{})
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			nowNum++
			_, err = cl.DoLimit(enet.NewReaderConn(c))
			if nowNum > 5 {
				assert.Error(t, err)
				close <- struct{}{}
			} else {
				assert.NoError(t, err)
			}
		}
	}()
	for i := 6; i > 0; i-- {
		go net.Dial("tcp", ln.Addr().String())
	}
	<-close
}
