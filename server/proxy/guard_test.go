package proxy

import (
	"testing"

	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
)

// 高危端口默认要带上连接频率限制，否则公网爆破会一直打到内网 3389/22 上
func TestDefaultRiskyPortRate(t *testing.T) {
	cases := map[int]int{
		21:    4,
		22:    4,
		3389:  4,
		445:   4,
		3306:  4,
		8080:  0, //普通端口不受影响
		23389: 0,
	}
	for port, want := range cases {
		if got := defaultRiskyPortRate(port); got != want {
			t.Fatalf("port %d: got %d, want %d", port, got, want)
		}
	}
}

// 全局开关与速率都要能从 nps.conf 覆盖
func TestRiskyPortRateConfig(t *testing.T) {
	if err := beego.AppConfig.Set("risky_port_conn_rate", "9"); err != nil {
		t.Fatalf("set rate: %v", err)
	}
	if got := defaultRiskyPortRate(3389); got != 9 {
		t.Fatalf("risky_port_conn_rate not applied, got %d", got)
	}
	if err := beego.AppConfig.Set("risky_port_guard", "false"); err != nil {
		t.Fatalf("set guard: %v", err)
	}
	if got := defaultRiskyPortRate(3389); got != 0 {
		t.Fatalf("risky_port_guard=false not applied, got %d", got)
	}
	//恢复默认，避免影响同包其它测试
	_ = beego.AppConfig.Set("risky_port_guard", "true")
	_ = beego.AppConfig.Set("risky_port_conn_rate", "")
}

// 3389 之类的隧道即使没配任何过滤条件，也要拿到默认的限速守卫
func TestTaskGuardHighRiskPort(t *testing.T) {
	task := &file.Tunnel{Id: 987654, Port: 3389}
	guard := TaskGuard(task)
	defer DropGuardCache(task.Id)
	if guard == nil || guard.Disabled() {
		t.Fatal("high risk port should get a guard by default")
	}
	if guard.RateLimit() != 4 {
		t.Fatalf("guard rate = %d, want 4", guard.RateLimit())
	}

	//普通端口不加默认限制，保持原有行为
	normal := &file.Tunnel{Id: 987655, Port: 18080}
	defer DropGuardCache(normal.Id)
	if g := TaskGuard(normal); g == nil || !g.Disabled() {
		t.Fatal("normal port should not get a default guard")
	}

	//隧道自己配了速率时以隧道为准
	custom := &file.Tunnel{Id: 987656, Port: 3389, ConnRate: 20}
	defer DropGuardCache(custom.Id)
	if g := TaskGuard(custom); g == nil || g.RateLimit() != 20 {
		t.Fatalf("tunnel conn rate override lost")
	}
}
