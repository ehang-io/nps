//go:build !linux
// +build !linux

package nps_mux

import "net"

// tuneTCP is Linux only: TCP_NOTSENT_LOWAT and per-socket congestion control
// are not portable, and a tunnel works without them.
func tuneTCP(net.Conn) {}
