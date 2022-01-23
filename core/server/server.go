package server

import "ehang.io/nps/core/handler"

type rule interface {
	handler.RuleRun
	GetHandler() handler.Handler
}

type Server interface {
	Init() error
	Serve()
	GetServerAddr() string
	GetName() string
	GetZhName() string
	RegisterHandle(rl rule)
}

type BaseServer struct {
	handlers   map[string]handler.Handler
}

func (bs *BaseServer) RegisterHandle(rl rule) {
	var h handler.Handler
	var ok bool
	if h, ok = bs.handlers[rl.GetHandler().GetName()]; !ok {
		h = rl.GetHandler()
		bs.handlers[h.GetName()] = h
	}
	h.AddRule(rl)
	return
}