package socks5

import (
	"context"
	"testing"
)

func TestNewS5Server(t *testing.T) {
	g := make(map[string]string)
	c := make(map[string]string)
	p := make(map[string]string)
	p["socks5_check_access"] = "true"
	p["socks5_simple_access_check"] = "true"
	p["socks5_simple_access_username"] = "111"
	p["socks5_simple_access_password"] = "222"
	s5 := NewS5Server(g, c, p, "", 1099)
	ctx := context.Background()
	s5.Start(ctx)
}
