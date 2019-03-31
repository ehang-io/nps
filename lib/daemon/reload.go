// +build !windows

package daemon

import (
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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
