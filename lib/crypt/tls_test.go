package crypt

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
)

func TestServerTlsConfigBuiltin(t *testing.T) {
	cfg, err := ServerTlsConfig("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("want 1 certificate, got %d", len(cfg.Certificates))
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("want TLS1.2 as the minimum version, got %d", cfg.MinVersion)
	}
}

func TestServerTlsConfigFallsBackWhenCertMissing(t *testing.T) {
	if _, err := LoadCertFile("no-such-cert.pem", "no-such-key.pem"); err == nil {
		t.Fatal("want an error for a missing certificate file")
	}
	cfg, err := ServerTlsConfig("no-such-cert.pem", "no-such-key.pem")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("want the built-in certificate as fallback, got %d certs", len(cfg.Certificates))
	}
}

func TestBuiltinCertHasSan(t *testing.T) {
	cert := BuiltinCert()
	if len(cert.Certificate) == 0 {
		t.Fatal("empty built-in certificate")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if !containsIP(leaf.IPAddresses, "127.0.0.1") {
		t.Fatalf("built-in certificate has no 127.0.0.1 SAN: %v", leaf.IPAddresses)
	}
	var hasLocalhost bool
	for _, name := range leaf.DNSNames {
		if name == "localhost" {
			hasLocalhost = true
		}
	}
	if !hasLocalhost {
		t.Fatalf("built-in certificate has no localhost SAN: %v", leaf.DNSNames)
	}
}

func TestClientTlsConfig(t *testing.T) {
	cfg := ClientTlsConfig("nps.example.com:8024", false)
	if cfg.ServerName != "nps.example.com" {
		t.Fatalf("want server name from the address, got %q", cfg.ServerName)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("self-signed deployment should skip verification by default")
	}
	strict := ClientTlsConfig("10.0.0.1:8024", true)
	if strict.ServerName != "10.0.0.1" {
		t.Fatalf("want ip as server name, got %q", strict.ServerName)
	}
	if strict.InsecureSkipVerify {
		t.Fatal("tls_verify=true must verify the certificate chain")
	}
}

func containsIP(ips []net.IP, want string) bool {
	for _, ip := range ips {
		if ip.String() == want {
			return true
		}
	}
	return false
}
