package rule

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/limiter"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/server"
	"encoding/json"
	"github.com/pkg/errors"
	"reflect"
)

type JsonData struct {
	ObjType string `json:"obj_type"`
	ObjData string `json:"obj_data"`
}

type JsonRule struct {
	Name     string     `json:"name"`
	Uuid     string     `json:"uuid"`
	Status   int        `json:"status"`
	Extend   int        `json:"extend"`
	Server   JsonData   `json:"server"`
	Handler  JsonData   `json:"handler"`
	Process  JsonData   `json:"process"`
	Action   JsonData   `json:"action"`
	Limiters []JsonData `json:"limiters"`
	Remark   string     `json:"remark"`
}

var NotFoundError = errors.New("not found")

func (jd *JsonRule) ToRule() (*Rule, error) {
	r := &Rule{Limiters: make([]limiter.Limiter, 0)}
	s, ok := chains[jd.Server.ObjType]
	if !ok {
		return nil, NotFoundError
	}
	r.Server = clone(s.Self).(server.Server)
	err := json.Unmarshal([]byte(jd.Server.ObjData), r.Server)
	if err != nil {
		return nil, err
	}
	h, ok := s.Children[jd.Handler.ObjType]
	if !ok {
		return nil, NotFoundError
	}
	r.Handler = clone(h.Self).(handler.Handler)
	err = json.Unmarshal([]byte(jd.Handler.ObjData), r.Handler)
	if err != nil {
		return nil, err
	}
	p, ok := h.Children[jd.Process.ObjType]
	if !ok {
		return nil, NotFoundError
	}
	r.Process = clone(p.Self).(process.Process)
	err = json.Unmarshal([]byte(jd.Process.ObjData), r.Process)
	if err != nil {
		return nil, err
	}
	a, ok := p.Children[jd.Action.ObjType]
	if !ok {
		return nil, NotFoundError
	}
	r.Action = clone(a.Self).(action.Action)
	err = json.Unmarshal([]byte(jd.Action.ObjData), r.Action)
	if err != nil {
		return nil, err
	}
	for _, v := range jd.Limiters {
		l, ok := limiters[v.ObjType]
		if !ok {
			return nil, NotFoundError
		}
		lm := clone(l.Self).(limiter.Limiter)
		err = json.Unmarshal([]byte(v.ObjData), lm)
		if err != nil {
			return nil, err
		}
		r.Limiters = append(r.Limiters, lm)
	}
	return r, nil
}

func clone(i interface{}) interface{} {
	v := reflect.ValueOf(i).Elem()
	vNew := reflect.New(v.Type())
	vNew.Elem().Set(v)
	return vNew.Interface()
}
