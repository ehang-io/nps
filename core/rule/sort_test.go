package rule

import (
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/process"
	"sort"
	"testing"
)

func TestSort_Len(t *testing.T) {
	r1 := &Rule{Handler: &handler.DefaultHandler{}, Process: &process.TransparentProcess{}}
	r2 := &Rule{Handler: &handler.DefaultHandler{}, Process: &process.DefaultProcess{}}
	r3 := &Rule{Handler: &handler.DefaultHandler{}, Process: &process.HttpServeProcess{RouteUrl: "/test/aaa"}}
	r4 := &Rule{Handler: &handler.DefaultHandler{}, Process: &process.Socks5Process{}}
	r5 := &Rule{Handler: &handler.DefaultHandler{}, Process: &process.HttpServeProcess{RouteUrl: "/test"}}
	r6 := &Rule{Handler: &handler.HttpsHandler{}, Process: &process.HttpsProxyProcess{}}
	s := make(Sort, 0)
	s = append(s, r1, r2, r3, r4, r5, r6)
	sort.Sort(s)
	expected := make(Sort, 0)
	expected = append(expected, r6, r5, r3, r4, r1, r2)
	for k, v := range expected {
		if v != s[k] {
			t.Fail()
		}
	}
}
