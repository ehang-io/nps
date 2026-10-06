package config

import "testing"

func TestParseLinks(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"off", -1}, {"OFF", -1}, {"false", -1}, {"no", -1},
		{"1", 1}, {"2", 2}, {" 8 ", 8},
		{"0", 0}, // not a way to turn them off: the default
		{"9", 0}, {"-3", 0}, {"two", 0}, {"", 0},
	} {
		if got := parseLinks(c.in); got != c.want {
			t.Errorf("parseLinks(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
