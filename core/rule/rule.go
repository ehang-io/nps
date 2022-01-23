package rule

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/limiter"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/server"
	"ehang.io/nps/lib/enet"
	"github.com/pkg/errors"
)

type Rule struct {
	Server   server.Server     `json:"server"`
	Handler  handler.Handler   `json:"handler"`
	Process  process.Process   `json:"process"`
	Action   action.Action     `json:"action"`
	Limiters []limiter.Limiter `json:"limiters"`
}

var servers map[string]server.Server

func init() {
	servers = make(map[string]server.Server, 0)
}

func (r *Rule) GetHandler() handler.Handler {
	return r.Handler
}

func (r *Rule) Init() error {
	s := r.Server
	var ok bool
	if s, ok = servers[r.Server.GetName()+":"+r.Server.GetServerAddr()]; !ok {
		s = r.Server
		err := s.Init()
		servers[r.Server.GetName()+":"+r.Server.GetServerAddr()] = s
		if err != nil {
			return err
		}
		go s.Serve()
	}
	s.RegisterHandle(r)
	r.Handler.AddRule(r)
	if err := r.Action.Init(); err != nil {
		return err
	}
	for _, l := range r.Limiters {
		if err := l.Init(); err != nil {
			return err
		}
	}
	return r.Process.Init(r.Action)
}

func (r *Rule) RunConn(c enet.Conn) (bool, error) {
	var err error
	for _, lm := range r.Limiters {
		if c, err = lm.DoLimit(c); err != nil {
			return true, errors.Wrap(err, "rule run")
		}
	}
	if err = c.Reset(0); err != nil {
		return false, err
	}
	return r.Process.ProcessConn(c)
}

func (r *Rule) RunPacketConn(pc enet.PacketConn) (bool, error) {
	return r.Process.ProcessPacketConn(pc)
}
