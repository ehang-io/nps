package file

import (
	"net/http"
	"testing"
)

func newTestDb(t *testing.T) *DbUtils {
	t.Helper()
	return &DbUtils{JsonDb: NewJsonDb(t.TempDir())}
}

func TestGetInfoByHostWildcardBoundary(t *testing.T) {
	db := newTestDb(t)
	db.JsonDb.Hosts.Store(1, &Host{Id: 1, Host: "*.proxy.com", Location: "/", Scheme: "all"})

	r, err := http.NewRequest("GET", "http://a.proxy.com/index", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.URL.Scheme = "http"
	r.RequestURI = "/index"

	if _, err := db.GetInfoByHost("a.proxy.com", r); err != nil {
		t.Fatalf("a.proxy.com 应当命中泛域名规则: %v", err)
	}
	//不能因为子串包含 proxy.com 就命中（域名混淆）
	if _, err := db.GetInfoByHost("evilproxy.com", r); err == nil {
		t.Error("evilproxy.com 不应当命中 *.proxy.com")
	}
}

func TestGetInfoByHostExactAndLocation(t *testing.T) {
	db := newTestDb(t)
	db.JsonDb.Hosts.Store(1, &Host{Id: 1, Host: "app.example.com", Location: "/", Scheme: "all"})
	db.JsonDb.Hosts.Store(2, &Host{Id: 2, Host: "app.example.com", Location: "/api", Scheme: "all"})

	r, err := http.NewRequest("GET", "http://app.example.com/api/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.URL.Scheme = "http"
	r.RequestURI = "/api/v1"

	h, err := db.GetInfoByHost("app.example.com", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	//应当命中最长前缀 /api
	if h.Id != 2 {
		t.Fatalf("host id = %d, want 2", h.Id)
	}

	//带端口的 Host 头需要正确去掉端口
	h2, err := db.GetInfoByHost("app.example.com:8080", r)
	if err != nil {
		t.Fatalf("带端口时应当仍能解析: %v", err)
	}
	if h2.Id != 2 {
		t.Fatalf("host id = %d, want 2", h2.Id)
	}
}
