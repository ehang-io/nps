package limiter

import (
	"ehang.io/nps/lib/enet"
)

var (
	_ Limiter = (*RateLimiter)(nil)
	_ Limiter = (*ConnNumLimiter)(nil)
	_ Limiter = (*IpConnNumLimiter)(nil)
	_ Limiter = (*FlowLimiter)(nil)
)

type Limiter interface {
	DoLimit(conn enet.Conn) (enet.Conn, error)
	Init() error
	GetName() string
	GetZhName() string
}

type baseLimiter struct {
}

func (bl *baseLimiter) Init() error {
	return nil
}
