//go:build !windows
// +build !windows

package selfupdate

import (
	"os"
	"syscall"
)

// restart replaces the running process image with bin, keeping the same pid,
// the same arguments and the same service unit.
//
// Exec is used rather than exiting and letting the supervisor start us again,
// because a unit with a RestartSec of a minute or two would turn every update
// -- and worse, every rollback -- into an outage of that length. Exec is
// immediate. File descriptors Go opened carry close-on-exec, so the tunnel
// sockets are dropped by the kernel as the new image takes over.
func restart(bin string) error {
	return syscall.Exec(bin, os.Args, os.Environ())
}
