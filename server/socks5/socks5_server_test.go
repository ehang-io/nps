package socks5

import (
	"context"
	"testing"
)

func TestNewS5Server(t *testing.T) {
	g := make(map[string]string)
	c := make(map[string]string)
	p := make(map[string]string)
	s5 := NewS5Server(g, c, p, "", 1099)
	ctx := context.Background()
	s5.Start(ctx)
}
