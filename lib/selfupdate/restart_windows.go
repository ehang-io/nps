//go:build windows
// +build windows

package selfupdate

import (
	"os"

	"github.com/astaxie/beego/logs"
)

// restart exits and leaves it to the service manager to start the new binary,
// since Windows has no exec that replaces the running image.
func restart(bin string) error {
	logs.Info("selfupdate: exiting so the service manager can start the new binary")
	os.Exit(0)
	return nil
}
