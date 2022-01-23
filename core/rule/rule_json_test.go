package rule

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/server"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestClone(t *testing.T) {
	type person struct {
		Name string
		Age  int
	}
	a := &person{
		Name: "ALice",
		Age:  20,
	}
	b := clone(a).(*person)
	assert.Equal(t, a.Name, b.Name)
	assert.Equal(t, a.Age, b.Age)
	a.Name = "Bob"
	a.Age = 21
	assert.NotEqual(t, a.Name, b.Name)
	assert.NotEqual(t, a.Age, b.Age)
	assert.NotEqual(t, reflect.ValueOf(a).Pointer(), reflect.ValueOf(b).Pointer())
}

func getJson(t *testing.T, i interface{}) string {
	b, err := json.Marshal(i)
	assert.NoError(t, err)
	assert.NotEmpty(t, string(b))
	return string(b)
}

func TestJsonRule(t *testing.T) {
	s := &server.TcpServer{ ServerAddr: "127.0.0.1:0"}
	h := &handler.HttpHandler{}
	p := &process.HttpServeProcess{}
	a := &action.LocalAction{}
	js := JsonRule{
		Uuid:     "",
		Server:   JsonData{s.GetName(), getJson(t, s)},
		Handler:  JsonData{h.GetName(), getJson(t, h)},
		Process:  JsonData{p.GetName(), getJson(t, p)},
		Action:   JsonData{a.GetName(), getJson(t, a)},
		Limiters: make([]JsonData, 0),
	}
	rl, err := js.ToRule()
	assert.NoError(t, err)
	err = rl.Init()
	assert.NoError(t, err)
	assert.Equal(t, rl.Server.(*server.TcpServer).ServerAddr, "127.0.0.1:0")
}
