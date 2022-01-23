package rule

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/limiter"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/server"
	"github.com/fatih/structtag"
	"reflect"
	"strconv"
)

var orderMap map[string]int
var nowOrder = 2<<8 - 1

type children map[string]*List

var chains children
var limiters children

func init() {
	orderMap = make(map[string]int, 0)
	chains = make(map[string]*List, 0)
	limiters = make(map[string]*List, 0)
	chains.Append(&server.TcpServer{}).Append(&handler.HttpHandler{}).Append(&process.HttpServeProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{}, &action.AdminAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.HttpsHandler{}).Append(&process.HttpsServeProcess{HttpServeProcess: process.HttpServeProcess{}}).AppendMany(&action.NpcAction{}, &action.LocalAction{}, &action.AdminAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.HttpsHandler{}).Append(&process.HttpsRedirectProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.HttpHandler{}).Append(&process.HttpProxyProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.HttpsHandler{}).Append(&process.HttpsProxyProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.Socks5Handler{}).Append(&process.Socks5Process{}).AppendMany(&action.LocalAction{}, &action.NpcAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.TransparentHandler{}).Append(&process.TransparentProcess{}).AppendMany(&action.NpcAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.RdpHandler{}).Append(&process.DefaultProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})
	chains.Append(&server.TcpServer{}).Append(&handler.RedisHandler{}).Append(&process.DefaultProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})

	chains.Append(&server.UdpServer{}).Append(&handler.DnsHandler{}).Append(&process.DefaultProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})
	// TODO p2p
	chains.Append(&server.UdpServer{}).Append(&handler.P2PHandler{}).Append(&process.DefaultProcess{}).AppendMany(&action.NpcAction{}, &action.LocalAction{})
	chains.Append(&server.UdpServer{}).Append(&handler.QUICHandler{}).Append(&process.DefaultProcess{}).AppendMany(&action.BridgeAction{})
	chains.Append(&server.UdpServer{}).Append(&handler.Socks5UdpHandler{}).Append(&process.Socks5Process{}).AppendMany(&action.LocalAction{}, &action.NpcAction{})

	chains.Append(&server.TcpServer{}).Append(&handler.DefaultHandler{}).Append(&process.DefaultProcess{}).AppendMany(&action.BridgeAction{}, &action.AdminAction{}, &action.NpcAction{}, &action.LocalAction{})

	limiters.AppendMany(&limiter.RateLimiter{}, &limiter.ConnNumLimiter{}, &limiter.FlowLimiter{}, &limiter.IpConnNumLimiter{})
}

func GetLimiters() children {
	return limiters
}

func GetChains() children {
	return chains
}

type NameInterface interface {
	GetName() string
	GetZhName() string
}

type List struct {
	ZhName   string      `json:"zh_name"`
	Self     interface{} `json:"-"`
	Field    []field     `json:"field"`
	Children children    `json:"children"`
}

func (c children) AppendMany(child ...NameInterface) {
	for _, cd := range child {
		c.Append(cd)
	}
}

func (c children) Append(child NameInterface) children {
	if v, ok := c[child.GetName()]; ok {
		return v.Children
	}
	if _, ok := orderMap[child.GetName()]; !ok {
		orderMap[child.GetName()] = nowOrder
		nowOrder--
	}
	cd := &List{Self: child, Field: getFieldName(child), Children: make(map[string]*List, 0), ZhName: child.GetZhName()}
	c[child.GetName()] = cd
	return cd.Children
}

type field struct {
	FiledType     string `json:"field_type"`
	FieldName     string `json:"field_name"`
	FieldZhName   string `json:"field_zh_name"`
	FieldRequired bool   `json:"field_required"`
	FieldExample  string `json:"field_example"`
}

func getFieldName(structName interface{}, child ...bool) []field {
	result := make([]field, 0)
	t := reflect.TypeOf(structName)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return result
	}
	fieldNum := t.NumField()
	for i := 0; i < fieldNum; i++ {
		if len(child) == 0 && t.Field(i).Type.Kind() == reflect.Struct {
			value := reflect.ValueOf(structName)
			if value.Kind() == reflect.Ptr {
				value = value.Elem()
			}
			if value.Field(i).CanInterface() {
				result = append(result, getFieldName(value.Field(i).Interface(), true)...)
			}
		}
		tags, err := structtag.Parse(string(t.Field(i).Tag))
		if err == nil {
			tag, err := tags.Get("json")
			if err == nil {
				f := field{}
				f.FiledType = t.Field(i).Type.Kind().String()
				f.FieldName = tag.Name
				tag, err = tags.Get("required")
				if err == nil {
					f.FieldRequired, _ = strconv.ParseBool(tag.Name)
				}
				tag, err = tags.Get("placeholder")
				if err == nil {
					f.FieldExample = tag.Name
				}
				tag, err = tags.Get("zh_name")
				if err == nil {
					f.FieldZhName = tag.Name
				}
				result = append(result, f)
			}
		}
	}
	return result
}
