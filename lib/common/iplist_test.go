package common

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestIPListContains(t *testing.T) {
	cases := []struct {
		raw  string
		ip   string
		want bool
	}{
		{"", "1.2.3.4", false},
		{"*", "1.2.3.4", true},
		{"1.2.3.4", "1.2.3.4", true},
		{"1.2.3.4", "1.2.3.5", false},
		{"10.0.0.0/8", "10.1.2.3", true},
		{"10.0.0.0/8", "11.1.2.3", false},
		{"192.168.1.*", "192.168.1.9", true},
		{"192.168.1.*", "192.168.2.9", false},
		{"1.2.3.4, 10.0.0.0/8", "10.9.9.9", true},
		{"1.2.3.4;10.0.0.0/8", "10.9.9.9", true},
		{"::1", "::1", true},
		{"2001:db8::/32", "2001:db8::1", true},
		{"not-an-ip", "1.2.3.4", false},
	}
	for _, c := range cases {
		l := NewIPList(c.raw)
		if got := l.Contains(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("NewIPList(%q).Contains(%q) = %v, want %v", c.raw, c.ip, got, c.want)
		}
	}
}

func TestIPListEmpty(t *testing.T) {
	if !NewIPList("").Empty() {
		t.Error("empty config should be empty list")
	}
	if !NewIPList("   ,  ").Empty() {
		t.Error("blank entries should be empty list")
	}
	if NewIPList("1.2.3.4").Empty() {
		t.Error("single ip should not be empty")
	}
}

func TestIPFromAddr(t *testing.T) {
	if ip := IPFromAddr(&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 9}); ip == nil || ip.String() != "1.2.3.4" {
		t.Errorf("tcp addr = %v, want 1.2.3.4", ip)
	}
	if ip := IPFromAddr(&stringAddr{"5.6.7.8:1234"}); ip == nil || ip.String() != "5.6.7.8" {
		t.Errorf("string addr = %v, want 5.6.7.8", ip)
	}
	if ip := IPFromAddr(&stringAddr{"[2001:db8::1]:80"}); ip == nil || ip.String() != "2001:db8::1" {
		t.Errorf("ipv6 addr = %v, want 2001:db8::1", ip)
	}
	if ip := IPFromAddr(nil); ip != nil {
		t.Errorf("nil addr = %v, want nil", ip)
	}
}

type stringAddr struct{ v string }

func (a *stringAddr) Network() string { return "tcp" }
func (a *stringAddr) String() string  { return a.v }

func TestAccessGuardAllowDeny(t *testing.T) {
	g := NewAccessGuard("10.0.0.0/8", "10.1.2.3", 0, time.Second, time.Minute)
	if ok, reason := g.Filter(&net.TCPAddr{IP: net.ParseIP("1.1.1.1")}); ok || reason != DenyReasonNotAllowed {
		t.Errorf("ip outside allow list should be rejected, got ok=%v reason=%q", ok, reason)
	}
	//黑名单优先级高于白名单
	if ok, reason := g.Filter(&net.TCPAddr{IP: net.ParseIP("10.1.2.3")}); ok || reason != DenyReasonDenyList {
		t.Errorf("ip in deny list should be rejected, got ok=%v reason=%q", ok, reason)
	}
	if ok, _ := g.Filter(&net.TCPAddr{IP: net.ParseIP("10.1.2.4")}); !ok {
		t.Error("ip in allow list should pass")
	}
	if g.Disabled() {
		t.Error("guard with allow list should not be disabled")
	}
}

func TestAccessGuardRateLimitAndBan(t *testing.T) {
	now := time.Unix(1700000000, 0)
	g := NewAccessGuard("", "", 2, time.Second, time.Minute)
	g.now = func() time.Time { return now }
	if g.Disabled() {
		t.Fatal("guard with rate limit should not be disabled")
	}
	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 1000}
	for i := 0; i < 2; i++ {
		if ok, reason := g.Check(addr); !ok {
			t.Fatalf("connection %d rejected unexpectedly: %s", i, reason)
		}
	}
	if ok, reason := g.Check(addr); ok || reason != DenyReasonRateLimit {
		t.Fatalf("third connection should hit rate limit, got ok=%v reason=%q", ok, reason)
	}
	now = now.Add(30 * time.Second)
	if ok, reason := g.Check(addr); ok || reason != DenyReasonBanned {
		t.Fatalf("source should stay banned, got ok=%v reason=%q", ok, reason)
	}
	//其他来源不受影响
	if ok, _ := g.Check(&net.TCPAddr{IP: net.ParseIP("5.6.7.8"), Port: 1000}); !ok {
		t.Fatal("other source should not be affected")
	}
	now = now.Add(31 * time.Second)
	if ok, reason := g.Check(addr); !ok {
		t.Fatalf("source should be released after ban expired, reason=%q", reason)
	}
}

func TestAccessGuardDisabled(t *testing.T) {
	g := NewAccessGuard("", "", 0, time.Second, time.Minute)
	if !g.Disabled() {
		t.Error("guard without any rule should be disabled")
	}
	if ok, reason := g.Check(&net.TCPAddr{IP: net.ParseIP("1.2.3.4")}); !ok {
		t.Errorf("disabled guard must allow, reason=%q", reason)
	}
	var nilGuard *AccessGuard
	if ok, _ := nilGuard.Check(&net.TCPAddr{IP: net.ParseIP("1.2.3.4")}); !ok {
		t.Error("nil guard must allow")
	}
}

func TestChangeHostAndHeaderTrimsForwardedChain(t *testing.T) {
	r, err := http.NewRequest("GET", "http://origin.internal/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header = http.Header{}
	prior := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		prior = append(prior, "10.0.0.1")
	}
	r.Header.Set("X-Forwarded-For", strings.Join(prior, ", "))
	ChangeHostAndHeader(r, "", "", "1.2.3.4:5555", true)

	chain := r.Header.Get("X-Forwarded-For")
	items := strings.Split(chain, ",")
	if len(items) != maxForwardedForEntries+1 {
		t.Fatalf("forwarded chain length = %d, want %d", len(items), maxForwardedForEntries+1)
	}
	if got := strings.TrimSpace(items[len(items)-1]); got != "1.2.3.4" {
		t.Errorf("last hop = %q, want 1.2.3.4", got)
	}
	if got := r.Header.Get("X-Real-IP"); got != "1.2.3.4" {
		t.Errorf("X-Real-IP = %q, want 1.2.3.4", got)
	}

	//addOrigin=false 时不写入来源头
	r2, _ := http.NewRequest("GET", "http://origin.internal/", nil)
	r2.Header = http.Header{}
	ChangeHostAndHeader(r2, "", "", "1.2.3.4:5555", false)
	if got := r2.Header.Get("X-Forwarded-For"); got != "" {
		t.Errorf("X-Forwarded-For = %q, want empty", got)
	}
}

func TestChangeHostAndHeaderIPv6Source(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://origin.internal/", nil)
	r.Header = http.Header{}
	ChangeHostAndHeader(r, "", "", "[2001:db8::1]:5555", true)
	if got := r.Header.Get("X-Real-IP"); got != "2001:db8::1" {
		t.Errorf("X-Real-IP = %q, want 2001:db8::1", got)
	}
}
