package ftp

import (
	"reflect"
	"testing"
)

func TestVerb(t *testing.T) {
	cases := map[string]string{
		"PORT 10,0,0,1,10,10\r\n":                  "PORT",
		"227 Entering Passive Mode (10,0,0,1,1,2)": "227",
		"221\r\n":  "221",
		"":         "",
		"  user a": "USER",
	}
	for in, want := range cases {
		if got := Verb(in); got != want {
			t.Errorf("Verb(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSetPasv(t *testing.T) {
	line := "227 Entering Passive Mode (192,168,1,5,19,136).\r\n"
	host, port, ok := ParsePasv(line)
	if !ok || host != "192.168.1.5" || port != 19<<8|136 {
		t.Fatalf("ParsePasv = %q,%d,%v", host, port, ok)
	}
	out, ok := SetPasv(line, "203.0.113.7", 40001)
	if !ok {
		t.Fatal("SetPasv failed")
	}
	want := "227 Entering Passive Mode (203,0,113,7,156,65).\r\n"
	if out != want {
		t.Fatalf("SetPasv = %q, want %q", out, want)
	}
	if _, _, ok := ParsePasv("227-Entering Passive Mode\r\n"); ok {
		t.Fatal("multiline 227 without address must not parse")
	}
	if _, _, ok := ParsePasv("227 Entering Passive Mode (1,2,3)\r\n"); ok {
		t.Fatal("short address must not parse")
	}
	if _, _, ok := ParsePasv("227 Entering Passive Mode (1,2,3,4,300,1)\r\n"); ok {
		t.Fatal("out of range byte must not parse")
	}
	if _, ok := SetPasv(line, "not-an-ip", 40001); ok {
		t.Fatal("non-ip host must not be accepted")
	}
	if _, ok := SetPasv(line, "2001:db8::1", 40001); ok {
		t.Fatal("ipv6 host cannot be encoded in PASV")
	}
}

func TestParseSetEpsv(t *testing.T) {
	line := "229 Entering Extended Passive Mode (|||6446|)\r\n"
	port, ok := ParseEpsv(line)
	if !ok || port != 6446 {
		t.Fatalf("ParseEpsv = %d,%v", port, ok)
	}
	out, ok := SetEpsv(line, 40002)
	if !ok || out != "229 Entering Extended Passive Mode (|||40002|)\r\n" {
		t.Fatalf("SetEpsv = %q,%v", out, ok)
	}
	if _, ok := ParseEpsv("229 Entering Extended Passive Mode (|||0|)\r\n"); ok {
		t.Fatal("zero port must not parse")
	}
	if _, ok := ParseEpsv("227 Entering Passive Mode (1,2,3,4,1,2)\r\n"); ok {
		t.Fatal("227 must not be treated as 229")
	}
}

func TestParseSetPort(t *testing.T) {
	line := "PORT 192,168,1,10,19,136\r\n"
	addr, ok := ParsePort(line)
	if !ok || addr != "192.168.1.10:5000" {
		t.Fatalf("ParsePort = %q,%v", addr, ok)
	}
	out, ok := SetPort(line, "203.0.113.7", 40003)
	if !ok || out != "PORT 203,0,113,7,156,67\r\n" {
		t.Fatalf("SetPort = %q,%v", out, ok)
	}
	if _, ok := ParsePort("port 10,0,0,1,1\r\n"); ok {
		t.Fatal("incomplete address must not parse")
	}
	if _, ok := SetPort("PORT", "203.0.113.7", 1); ok {
		t.Fatal("PORT without args must not be rewritten")
	}
}

func TestParseSetEprt(t *testing.T) {
	addr, ok := ParseEprt("EPRT |1|192.168.1.10|5000|\r\n")
	if !ok || addr != "192.168.1.10:5000" {
		t.Fatalf("ParseEprt = %q,%v", addr, ok)
	}
	addr, ok = ParseEprt("EPRT |2|2001:db8::1|5000|\r\n")
	if !ok || addr != "[2001:db8::1]:5000" {
		t.Fatalf("ParseEprt ipv6 = %q,%v", addr, ok)
	}
	out, ok := SetEprt("EPRT |2|2001:db8::1|5000|\r\n", "203.0.113.7", 40004)
	if !ok || out != "EPRT |1|203.0.113.7|40004|\r\n" {
		t.Fatalf("SetEprt = %q,%v", out, ok)
	}
	if _, ok := ParseEprt("EPRT |3|1.2.3.4|1|\r\n"); ok {
		t.Fatal("unknown protocol must not parse")
	}
	if _, ok := ParseEprt("EPRT |1|1.2.3.4|\r\n"); ok {
		t.Fatal("missing port must not parse")
	}
}

func TestIsAuthTls(t *testing.T) {
	for _, line := range []string{"AUTH TLS\r\n", "auth ssl\r\n", "AUTH tls"} {
		if !IsAuthTls(line) {
			t.Fatalf("%q should be detected as AUTH TLS", line)
		}
	}
	for _, line := range []string{"AUTH GSSAPI\r\n", "USER auth\r\n", "AUTH\r\n"} {
		if IsAuthTls(line) {
			t.Fatalf("%q should not be AUTH TLS", line)
		}
	}
}

func TestParsePortsSpec(t *testing.T) {
	got := ParsePortsSpec("40000-40002,40005, 40001")
	want := []int{40000, 40001, 40002, 40005}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParsePortsSpec = %v, want %v", got, want)
	}
	if got := ParsePortsSpec("99999,abc,0,-5"); len(got) != 0 {
		t.Fatalf("invalid spec should yield no ports, got %v", got)
	}
	if got := ParsePortsSpec(""); len(got) != 0 {
		t.Fatalf("empty spec should yield no ports, got %v", got)
	}
	// 范围过大时按 4096 截断，避免生成巨大的切片
	if got := ParsePortsSpec("1000-99999"); len(got) != 4097 {
		t.Fatalf("oversized range should be truncated, got %d ports", len(got))
	}
}
