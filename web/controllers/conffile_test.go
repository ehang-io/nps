package controllers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 就地替换：保留注释、其它配置项、缩进与 CRLF 行尾
func TestUpdateConfValueReplacesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nps.conf")
	src := "#comment auth_key=keep\r\n[auth]\r\n  auth_key=old\r\napi_key=oldapi\r\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateConfValue(path, "auth_key", "newkey123"); err != nil {
		t.Fatalf("updateConfValue: %v", err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "#comment auth_key=keep") {
		t.Fatalf("comment must be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "  auth_key=newkey123\r\n") {
		t.Fatalf("value not replaced in place (indent/CRLF lost), got:\n%s", got)
	}
	if len(strings.Split(got, "\r\n")) != 5 {
		t.Fatalf("line count changed unexpectedly, got:\n%s", got)
	}
}

// 只改目标 key，其它 key 保持原值
func TestUpdateConfValueKeepsOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nps.conf")
	if err := os.WriteFile(path, []byte("api_key=keepme\nauth_key=old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateConfValue(path, "auth_key", "n"+"e"+"w"); err != nil {
		t.Fatal(err)
	}
	got := readFileString(t, path)
	if !strings.Contains(got, "api_key=keepme\n") || !strings.Contains(got, "auth_key=new\n") {
		t.Fatalf("unexpected config:\n%s", got)
	}
}

// 没有该配置项时追加一行；空值写成 key=
func TestUpdateConfValueAppendsMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nps.conf")
	if err := os.WriteFile(path, []byte("[common]\nserver_addr=1.2.3.4"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateConfValue(path, "api_key", "abc"); err != nil {
		t.Fatal(err)
	}
	got := readFileString(t, path)
	if !strings.HasSuffix(got, "server_addr=1.2.3.4\napi_key=abc\n") {
		t.Fatalf("missing key must be appended with a leading newline, got:\n%q", got)
	}
	if err := updateConfValue(path, "api_key", ""); err != nil {
		t.Fatal(err)
	}
	if got = readFileString(t, path); !strings.Contains(got, "api_key=\n") {
		t.Fatalf("empty value must be written as key=, got:\n%q", got)
	}
}

// 文件不存在时按新建处理（全新部署、conf 目录还没生成）
func TestUpdateConfValueCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conf", "nps.conf")
	if err := updateConfValue(path, "auth_key", "k"); err != nil {
		t.Fatalf("updateConfValue on missing file: %v", err)
	}
	if got := readFileString(t, path); got != "auth_key=k\n" {
		t.Fatalf("unexpected content %q", got)
	}
}

// 空 key 直接拒绝，避免把配置写成 "=value"
func TestUpdateConfValueRejectsEmptyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nps.conf")
	if err := updateConfValue(path, "  ", "v"); err == nil {
		t.Fatal("empty key must be rejected")
	}
}

func TestValidateSettingKey(t *testing.T) {
	ok := []string{"abcdefgh", "A1_b2-c3", strings.Repeat("x", settingKeyMax)}
	for _, k := range ok {
		if err := validateSettingKey(k); err != nil {
			t.Fatalf("%q should be accepted: %v", k, err)
		}
	}
	bad := []string{"", "short", strings.Repeat("x", settingKeyMax+1), "has space", "a/b", "中文密钥很长的测试值"}
	for _, k := range bad {
		if err := validateSettingKey(k); err == nil {
			t.Fatalf("%q should be rejected", k)
		}
	}
}

func TestRandomSettingKey(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 16; i++ {
		k, err := randomSettingKey(32)
		if err != nil {
			t.Fatalf("randomSettingKey: %v", err)
		}
		if len(k) != 32 {
			t.Fatalf("key length = %d, want 32 (%q)", len(k), k)
		}
		if err := validateSettingKey(k); err != nil {
			t.Fatalf("generated key %q is not valid: %v", k, err)
		}
		if seen[k] {
			t.Fatalf("duplicate key generated: %q", k)
		}
		seen[k] = true
	}
}

func TestCurlExample(t *testing.T) {
	got := curlExample("http://127.0.0.1:8080", "abc123")
	want := `curl -s -H "X-Api-Key: abc123" http://127.0.0.1:8080/api/v1/tunnels`
	if got != want {
		t.Fatalf("curlExample = %q, want %q", got, want)
	}
	if got := curlExample("http://h", ""); !strings.Contains(got, "<api_key>") {
		t.Fatalf("placeholder expected when api_key is empty, got %q", got)
	}
}

func TestSignedLoginURL(t *testing.T) {
	if got := signedLoginURL("http://h", ""); got != "" {
		t.Fatalf("no auth_key must yield no url, got %q", got)
	}
	got := signedLoginURL("http://h:8080", "secretkey")
	if !strings.HasPrefix(got, "http://h:8080/login/index?auth_key=") || !strings.Contains(got, "&timestamp=") {
		t.Fatalf("unexpected signed url %q", got)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
