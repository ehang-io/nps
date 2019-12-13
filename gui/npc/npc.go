package main

import (
	"fmt"
	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/client"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/daemon"
	"github.com/cnlh/nps/lib/version"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

func main() {
	daemon.InitDaemon("npc", common.GetRunPath(), common.GetTmpPath())
	logs.SetLogger("store")
	application := app.New()
	window := application.NewWindow("Npc " + version.VERSION)
	window.SetContent(WidgetScreen())
	window.Resize(fyne.NewSize(910, 350))

	window.ShowAndRun()

}

var (
	start    bool
	status   = "Start!"
	connType = "tcp"
	cl       = new(client.TRPClient)
)

func WidgetScreen() fyne.CanvasObject {
	return fyne.NewContainerWithLayout(layout.NewBorderLayout(nil, nil, nil, nil),
		makeMainTab(),
	)
}

func makeMainTab() fyne.Widget {
	serverPort := widget.NewEntry()
	serverPort.SetPlaceHolder("Server:Port")

	vKey := widget.NewEntry()
	vKey.SetPlaceHolder("Vkey")

	radio := widget.NewRadio([]string{"tcp", "kcp"}, func(s string) { connType = s })
	radio.Horizontal = true

	refreshCh := make(chan struct{})
	button := widget.NewButton(status, func() {
		start = !start
		if start {
			status = "Stop!"
			// init the npc
			fmt.Println("submit", serverPort.Text, vKey.Text, connType)
			sp, vk, ct := loadConfig()
			if sp != serverPort.Text || vk != vKey.Text || ct != connType {
				saveConfig(serverPort.Text, vKey.Text, connType)
			}
			cl = client.NewRPClient(serverPort.Text, vKey.Text, connType, "", nil)
			go cl.Start()
		} else {
			// close the npc
			status = "Start!"
			if cl != nil {
				go cl.Close()
				cl = nil
			}
		}
		refreshCh <- struct{}{}
	})
	go func() {
		for {
			<-refreshCh
			button.SetText(status)
		}
	}()

	lo := widget.NewMultiLineEntry()
	lo.SetReadOnly(true)
	lo.Resize(fyne.NewSize(910, 250))
	slo := widget.NewScrollContainer(lo)
	slo.Resize(fyne.NewSize(910, 250))
	go func() {
		for {
			time.Sleep(time.Second)
			lo.SetText(common.GetLogMsg())
			slo.Resize(fyne.NewSize(910, 250))
		}
	}()

	sp, vk, ct := loadConfig()
	if sp != "" && vk != "" && ct != "" {
		serverPort.SetText(sp)
		vKey.SetText(vk)
		connType = ct
		radio.SetSelected(ct)
	}

	return widget.NewVBox(
		widget.NewLabel("Npc "+version.VERSION),
		serverPort,
		vKey,
		radio,
		button,
		slo,
	)
}

func getDir() (dir string, err error) {
	if runtime.GOOS != "android" {
		dir, err = os.UserConfigDir()
		if err != nil {
			return
		}
	} else {
		dir, err = os.UserHomeDir()
		if err != nil {
			return
		}
		dir = path.Dir(dir)
		dir = path.Join(dir, "files")
	}
	return
}

func saveConfig(host, vkey, connType string) {
	data := strings.Join([]string{host, vkey, connType}, "\n")
	ph, err := getDir()
	if err != nil {
		logs.Warn("not found config dir")
		return
	}
	_ = os.Remove(path.Join(ph, "npc.conf"))
	f, err := os.OpenFile(path.Join(ph, "npc.conf"), os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	if err != nil {
		logs.Error(err)
		return
	}
	if _, err := f.Write([]byte(data)); err != nil {
		f.Close() // ignore error; Write error takes precedence
		logs.Error(err)
		return
	}
}

func loadConfig() (host, vkey, connType string) {
	ph, err := getDir()
	if err != nil {
		logs.Warn("not found config dir")
		return
	}
	f, err := os.OpenFile(path.Join(ph, "npc.conf"), os.O_RDONLY, 0644)
	defer f.Close()
	if err != nil {
		logs.Error(err)
		return
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		logs.Error(err)
		return
	}
	li := strings.Split(string(data), "\n")
	host = li[0]
	vkey = li[1]
	connType = li[2]
	return
}
