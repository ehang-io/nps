package config

import (
	"log"
	"regexp"
	"testing"
)

func TestReg(t *testing.T) {
	content := `
[common]
server=127.0.0.1:8284
tp=tcp
vkey=123
[web2]
host=www.baidu.com
host_change=www.sina.com
target=127.0.0.1:8080,127.0.0.1:8082
header_cookkile=122123
header_user-Agent=122123
[web2]
host=www.baidu.com
host_change=www.sina.com
target=127.0.0.1:8080,127.0.0.1:8082
header_cookkile="122123"
header_user-Agent=122123
[tunnel1]
type=udp
target=127.0.0.1:8080
port=9001
compress=snappy
crypt=true
u=1
p=2
[tunnel2]
type=tcp
target=127.0.0.1:8080
port=9001
compress=snappy
crypt=true
u=1
p=2
`
	re, err := regexp.Compile(`\[.+?\]`)
	if err != nil {
		t.Fail()
	}
	log.Println(re.FindAllString(content, -1))
}

func TestDealCommon(t *testing.T) {
	// the config keys are server_addr / conn_type, the old test used the
	// legacy server / tp names and could never pass
	s := `server_addr=127.0.0.1:8284
conn_type=tcp
vkey=123`
	c := dealCommon(s)
	if c.Server != "127.0.0.1:8284" {
		t.Fatalf("server = %q", c.Server)
	}
	if c.Tp != "tcp" {
		t.Fatalf("conn_type = %q", c.Tp)
	}
	if c.VKey != "123" {
		t.Fatalf("vkey = %q", c.VKey)
	}
	if c.Client == nil || c.Client.Cnf == nil {
		t.Fatal("client config was not initialised")
	}
}

func TestDealCommonMultiServer(t *testing.T) {
	// 多服务端穿透：server_addr 支持逗号 / 分号 / 空格分隔
	cases := []struct {
		addr string
		want []string
	}{
		{"a.com:8024,b.com:8024", []string{"a.com:8024", "b.com:8024"}},
		{" a.com:8024 ; b.com:8024 ", []string{"a.com:8024", "b.com:8024"}},
		{"a.com:8024 b.com:8024\tc.com:8024", []string{"a.com:8024", "b.com:8024", "c.com:8024"}},
		{"a.com:8024", []string{"a.com:8024"}},
		{"", nil},
	}
	for _, cs := range cases {
		got := SplitServerAddr(cs.addr)
		if len(got) != len(cs.want) {
			t.Fatalf("SplitServerAddr(%q) = %v, want %v", cs.addr, got, cs.want)
		}
		for i := range got {
			if got[i] != cs.want[i] {
				t.Fatalf("SplitServerAddr(%q) = %v, want %v", cs.addr, got, cs.want)
			}
		}
	}

	c := dealCommon("server_addr=a.com:8024,b.com:8024\nconn_type=tcp\nvkey=123")
	if len(c.Servers) != 2 || c.Servers[0] != "a.com:8024" || c.Servers[1] != "b.com:8024" {
		t.Fatalf("servers = %v", c.Servers)
	}
	// Server 保持第一个地址，兼容 status 查询等只认单地址的逻辑
	if c.Server != "a.com:8024" {
		t.Fatalf("server = %q", c.Server)
	}
}

func TestGetTitleContent(t *testing.T) {
	s := "[common]"
	if getTitleContent(s) != "common" {
		t.Fail()
	}
}
