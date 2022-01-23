package limiter

import (
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/rate"
)

// RateLimiter is used to limit the speed of transport
type RateLimiter struct {
	baseLimiter
	RateLimit int64 `json:"rate_limit" required:"true" placeholder:"10(kb)" zh_name:"最大速度"`
	rate      *rate.Rate
}

func (rl *RateLimiter) GetName() string {
	return "rate"
}

func (rl *RateLimiter) GetZhName() string {
	return "带宽限制"
}

// Init the rate controller
func (rl *RateLimiter) Init() error {
	if rl.RateLimit > 0 && rl.rate == nil {
		rl.rate = rate.NewRate(rl.RateLimit)
		rl.rate.Start()
	}
	return nil
}

// DoLimit return limited Conn
func (rl *RateLimiter) DoLimit(c enet.Conn) (enet.Conn, error) {
	return NewRateConn(c, rl.rate), nil
}

// rateConn is used to limiter the rate fo connection
type rateConn struct {
	enet.Conn
	rate *rate.Rate
}

// NewRateConn return limited connection by rate interface
func NewRateConn(rc enet.Conn, rate *rate.Rate) enet.Conn {
	return &rateConn{
		Conn: rc,
		rate: rate,
	}
}

// Read data and remove capacity from rate pool
func (s *rateConn) Read(b []byte) (n int, err error) {
	n, err = s.Conn.Read(b)
	if s.rate != nil && err == nil {
		err = s.rate.Get(int64(n))
	}
	return
}

// Write data and remove capacity from rate pool
func (s *rateConn) Write(b []byte) (n int, err error) {
	n, err = s.Conn.Write(b)
	if s.rate != nil && err == nil {
		err = s.rate.Get(int64(n))
	}
	return
}
