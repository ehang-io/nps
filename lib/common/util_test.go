package common

import (
	"encoding/base64"
	"net/http"
	"testing"
)

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestDomainCheck(t *testing.T) {
	cases := map[string]bool{
		"example.com":          true,
		"http://example.com":   true,
		"sub.example.com/path": true,
		"example.technology":   true, //长 TLD，原实现限制为 2~6 个字符会误判
		"127.0.0.1:8080":       false,
		"localhost":            false,
		"":                     false,
	}
	for in, want := range cases {
		if got := DomainCheck(in); got != want {
			t.Errorf("DomainCheck(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsPort(t *testing.T) {
	valid := []string{"1", "80", "65535"}
	invalid := []string{"0", "65536", "-1", "abc", "80000"}

	for _, p := range valid {
		if !IsPort(p) {
			t.Errorf("IsPort(%q) = false, want true", p)
		}
	}
	for _, p := range invalid {
		if IsPort(p) {
			t.Errorf("IsPort(%q) = true, want false", p)
		}
	}
}

func TestGetIpAndPortByAddr(t *testing.T) {
	ipCases := map[string]string{
		"1.2.3.4:8080": "1.2.3.4",
		"[::1]:8080":   "::1",
		"2001:db8::1":  "2001:db8::1",
		"10.0.0.1":     "10.0.0.1",
	}
	for addr, want := range ipCases {
		if got := GetIpByAddr(addr); got != want {
			t.Errorf("GetIpByAddr(%q) = %q, want %q", addr, got, want)
		}
	}

	portCases := map[string]int{
		"1.2.3.4:8080": 8080,
		"[::1]:9090":   9090,
		"1.2.3.4":      0,
		"::1":          0,
	}
	for addr, want := range portCases {
		if got := GetPortByAddr(addr); got != want {
			t.Errorf("GetPortByAddr(%q) = %d, want %d", addr, got, want)
		}
	}
}

func TestChangeHostAndHeader(t *testing.T) {
	r, err := http.NewRequest("GET", "http://origin.internal/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header = http.Header{}
	//header 值里包含冒号时不能被截断
	ChangeHostAndHeader(r, "new.example.com", "X-Forwarded-Host: new.example.com:8443\nUser-Agent: nps-test", "1.2.3.4:5555", true)

	if r.Host != "new.example.com" {
		t.Errorf("host = %q, want new.example.com", r.Host)
	}
	if got := r.Header.Get("X-Forwarded-Host"); got != "new.example.com:8443" {
		t.Errorf("X-Forwarded-Host = %q, want new.example.com:8443", got)
	}
	if got := r.Header.Get("User-Agent"); got != "nps-test" {
		t.Errorf("User-Agent = %q, want nps-test", got)
	}
	if got := r.Header.Get("X-Forwarded-For"); got != "1.2.3.4" {
		t.Errorf("X-Forwarded-For = %q, want 1.2.3.4", got)
	}
}

func TestGetExtFromPath(t *testing.T) {
	cases := map[string]string{
		"/var/www/index.HTML": "html",
		"a/b/c.tar.gz":        "gz",
		"noext":               "",
		"dir.v2/file":         "",
	}
	for in, want := range cases {
		if got := GetExtFromPath(in); got != want {
			t.Errorf("GetExtFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetPorts(t *testing.T) {
	got := GetPorts("80,8000-8002,abc,70000")
	want := []int{80, 8000, 8001, 8002}
	if len(got) != len(want) {
		t.Fatalf("GetPorts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("GetPorts = %v, want %v", got, want)
		}
	}
}

func TestCheckAuth(t *testing.T) {
	r, err := http.NewRequest("GET", "http://127.0.0.1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Proxy-Authorization", basicAuth("user", "p@ss:word"))
	if !CheckAuth(r, "user", "p@ss:word") {
		t.Error("CheckAuth 应当接受正确的用户名密码")
	}
	if CheckAuth(r, "user", "wrong") {
		t.Error("CheckAuth 不应当接受错误的密码")
	}
}
