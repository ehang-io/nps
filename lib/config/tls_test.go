package config

import "testing"

func TestDealTunnelTlsAndAuth(t *testing.T) {
	task := dealTunnel("mode=tcp\nserver_port=10000\ntarget_addr=127.0.0.1:3389\nauth_user=visitor\nauth_pass=change-me\ntls=true\ntls_cert_file=conf/a.pem\ntls_key_file=conf/a.key")
	if task.Mode != "tcp" || task.Ports != "10000" {
		t.Fatalf("unexpected tunnel: %+v", task)
	}
	if !task.AuthEnabled() {
		t.Fatal("auth_user/auth_pass should enable authentication")
	}
	if task.AuthUser != "visitor" || task.AuthPass != "change-me" {
		t.Fatalf("credential not parsed: %q/%q", task.AuthUser, task.AuthPass)
	}
	if !task.TlsEnabled() {
		t.Fatal("tls=true should enable tls")
	}
	if task.TlsCertFile != "conf/a.pem" || task.TlsKeyFile != "conf/a.key" {
		t.Fatalf("certificate paths not parsed: %q/%q", task.TlsCertFile, task.TlsKeyFile)
	}
}

func TestDealTunnelWithoutAuth(t *testing.T) {
	task := dealTunnel("mode=tcp\nserver_port=10001\ntarget_addr=127.0.0.1:80")
	if task.AuthEnabled() || task.TlsEnabled() {
		t.Fatal("plain tunnel must not enable auth or tls")
	}
}

func TestDealCommonTlsVerify(t *testing.T) {
	c := dealCommon("server_addr=nps.example.com:8024\nconn_type=tls\ntls_verify=true\nvkey=123")
	if c.Tp != "tls" {
		t.Fatalf("conn_type = %q, want tls", c.Tp)
	}
	if !c.TlsVerify {
		t.Fatal("tls_verify=true should be parsed")
	}
	if len(c.Servers) != 1 || c.Servers[0] != "nps.example.com:8024" {
		t.Fatalf("servers = %v", c.Servers)
	}
}
