package nps_mux

import (
	"log"
	"net"
	"os"
	"sync"
	"syscall"
)

// tcpNotsentLowat is TCP_NOTSENT_LOWAT, which the syscall package predates.
const tcpNotsentLowat = 0x19

// notsentLowat bounds how much of the mux's output may sit in the kernel not
// yet sent. Frames beyond it wait in the write queue, where a frame from
// another stream can still be scheduled ahead of them; once in the kernel
// their order is fixed. Unbounded, the kernel takes a whole send buffer --
// hundreds of kilobytes on a lossy link -- and a keystroke waits behind it.
const notsentLowat = 16 * 1024

// congestionEnv overrides the congestion control the mux asks for: an
// algorithm name, or "system" to leave the host default alone.
const congestionEnv = "NPS_MUX_CONGESTION"

// defaultCongestion is used unless congestionEnv says otherwise. cubic halves
// its window on every loss, so random loss on a long path holds a tunnel to a
// megabit or two however fast the link is; bbr paces to the measured rate.
const defaultCongestion = "bbr"

var congestionWarnOnce sync.Once

// tuneTCP sets the options a mux wants on its underlying TCP connection. Any
// that the host refuses are logged and skipped: the tunnel still works
// without them, only with more queueing.
func tuneTCP(c net.Conn) {
	tc, ok := underlying(c).(*net.TCPConn)
	if !ok {
		return
	}
	raw, err := tc.SyscallConn()
	if err != nil {
		log.Println("mux: tune tcp:", err)
		return
	}
	algorithm := defaultCongestion
	if v, ok := os.LookupEnv(congestionEnv); ok {
		algorithm = v
	}
	_ = raw.Control(func(fd uintptr) {
		if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpNotsentLowat, notsentLowat); err != nil {
			log.Println("mux: set TCP_NOTSENT_LOWAT:", err)
		}
		if algorithm == "" || algorithm == "system" {
			return
		}
		if err := syscall.SetsockoptString(int(fd), syscall.IPPROTO_TCP, syscall.TCP_CONGESTION, algorithm); err != nil {
			congestionWarnOnce.Do(func() {
				log.Printf("mux: congestion control %q unavailable, using the system default (%v); load it with modprobe tcp_%s, or set %s=system to silence this",
					algorithm, err, algorithm, congestionEnv)
			})
		}
	})
}
