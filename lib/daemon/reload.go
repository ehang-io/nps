// +build !windows

package daemon

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/astaxie/beego"
	"github.com/cnlh/nps/lib/common"
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
