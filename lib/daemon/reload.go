// +build !windows

package daemon

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ehang.io/nps/lib/common"
	"github.com/astaxie/beego"
)

func init() {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGUSR1)
	go func() {
		for {
			<-s
			beego.LoadAppConfig("ini", filepath.Join(common.GetRunPath(), "conf", "nps.conf"))
		}
	}()
}
