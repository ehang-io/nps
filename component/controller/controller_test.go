package controller

import (
	"bytes"
	"crypto/x509/pkix"
	"ehang.io/nps/core/action"
	"ehang.io/nps/core/handler"
	"ehang.io/nps/core/process"
	"ehang.io/nps/core/rule"
	"ehang.io/nps/core/server"
	"ehang.io/nps/db"
	"ehang.io/nps/lib/cert"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestController(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:3500")
	assert.NoError(t, err)
	err = os.Remove(filepath.Join(os.TempDir(), "test_control.db"))
	d := db.NewSqliteDb(filepath.Join(os.TempDir(), "test_control.db"))
	err = d.Init()
	assert.NoError(t, err)
	assert.NoError(t, d.SetConfig("admin_user", "admin"))
	assert.NoError(t, d.SetConfig("admin_pass", "pass"))
	cg := cert.NewX509Generator(pkix.Name{
		Country:            []string{"cn"},
		Organization:       []string{"ehang"},
		OrganizationalUnit: []string{"nps"},
		Province:           []string{"beijing"},
		CommonName:         "nps",
		Locality:           []string{"beijing"},
	})
	assert.NoError(t, err)
	cert, key, err := cg.CreateRootCa()
	assert.NoError(t, err)
	go func() {
		err = StartController(ln, d, cert, key, "./web/static/", "./web/views/")
		assert.NoError(t, err)
	}()
	resp, err := doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/login"), "POST", `{"username": "admin","password": "pass"}`)
	assert.NoError(t, err)
	assert.Equal(t, int(gjson.Parse(resp).Get("code").Int()), 0)

	for i := 0; i < 18; i++ {
		resp, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/v1/cert"), "POST", fmt.Sprintf(`{"status":1,"name":"name_%d","cert_type": "client"}`, i))
		assert.NoError(t, err)
		assert.Equal(t, int(gjson.Parse(resp).Get("code").Int()), 0)
	}

	resp, err = doRequest(fmt.Sprintf("http://%s%s?page=%d&pageSize=%d", ln.Addr().String(), "/v1/cert/page", 4, 5), "GET", ``)
	assert.NoError(t, err)
	now := 2
	var lastUuid string
	assert.Equal(t, len(gjson.Parse(resp).Get("result.items").Array()), 3)
	for _, v := range gjson.Parse(resp).Get("result.items").Array() {
		assert.Equal(t, v.Get("name").String(), fmt.Sprintf(`name_%d`, now))
		lastUuid = v.Get("uuid").String()
		now--
	}

	resp, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/v1/cert"), "DELETE", fmt.Sprintf(`{"uuid":"%s"}`, lastUuid))
	assert.NoError(t, err)
	assert.Equal(t, int(gjson.Parse(resp).Get("code").Int()), 0)

	s := &server.TcpServer{ServerAddr: "127.0.0.1:0"}
	h := &handler.DefaultHandler{}
	p := &process.DefaultProcess{}
	a := &action.LocalAction{}
	rj := &rule.JsonRule{
		Name:     "test",
		Status:   1,
		Server:   rule.JsonData{ObjType: s.GetName(), ObjData: getJson(t, s)},
		Handler:  rule.JsonData{ObjType: h.GetName(), ObjData: getJson(t, h)},
		Process:  rule.JsonData{ObjType: p.GetName(), ObjData: getJson(t, p)},
		Action:   rule.JsonData{ObjType: a.GetName(), ObjData: getJson(t, a)},
		Limiters: []rule.JsonData{},
	}
	js := getJson(t, rj)
	for i := 0; i < 18; i++ {
		resp, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/v1/rule"), "POST", js)
		assert.NoError(t, err)
		assert.Equal(t, int(gjson.Parse(resp).Get("code").Int()), 0)
	}
	resp, err = doRequest(fmt.Sprintf("http://%s%s?page=%d&pageSize=%d", ln.Addr().String(), "/v1/rule/page", 1, 10), "GET", ``)
	assert.NoError(t, err)
	assert.Equal(t, int(gjson.Parse(resp).Get("result.total").Int()), 18)

	uuid := gjson.Parse(resp).Get("result.items").Array()[0].Get("uuid").String()
	resp, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/v1/rule"), "GET", fmt.Sprintf(`{"uuid": "%s"}`, uuid))
	assert.NoError(t, err)
	assert.Equal(t, gjson.Parse(resp).Get("result.uuid").String(), uuid)

	rj.Uuid = uuid
	resp, err = doRequest(fmt.Sprintf("http://%s%s", ln.Addr().String(), "/v1/rule"), "PUT", getJson(t, rj))
	assert.NoError(t, err)
	assert.Equal(t, int(gjson.Parse(resp).Get("code").Int()), 0)
	time.Sleep(time.Minute * 600)
}

func getJson(t *testing.T, i interface{}) string {
	b, err := json.Marshal(i)
	assert.NoError(t, err)
	assert.NotEmpty(t, string(b))
	return string(b)
}

var client *http.Client
var once sync.Once
var cookies []*http.Cookie

func doRequest(url string, method string, body string) (string, error) {
	once.Do(func() {
		client = &http.Client{}
	})
	payload := bytes.NewBufferString(body)
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if len(res.Cookies()) > 0 {
		cookies = res.Cookies()
	}
	if res.StatusCode != 200 {
		return string(b), errors.New("bad doRequest")
	}
	return string(b), err
}
