package lib

import (
	"github.com/astaxie/beego"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func InitDaemon(f string) {
	if len(os.Args) < 2 {
		return
	}
	var args []string
	args = append(args, os.Args[0])
	if len(os.Args) >= 2 {
		args = append(args, os.Args[2:]...)
	}
	args = append(args, "-log=file")
	switch os.Args[1] {
	case "start":
		start(args, f)
		os.Exit(0)
	case "stop":
		stop(f, args[0])
		os.Exit(0)
	case "restart":
		stop(f, args[0])
		start(args, f)
		os.Exit(0)
	case "install":
		InstallNps()
	}
}

func start(osArgs []string, f string) {
	cmd := exec.Command(osArgs[0], osArgs[1:]...)
	cmd.Start()
	log.Println("执行启动成功")
	if cmd.Process.Pid > 0 {
		d1 := []byte(strconv.Itoa(cmd.Process.Pid))
		ioutil.WriteFile(beego.AppPath+"/"+f+".pid", d1, 0600)
	}
}

func stop(f string, p string) {
	var c *exec.Cmd
	var err error
	switch runtime.GOOS {
	case "windows":
		p := strings.Split(p, `\`)
		c = exec.Command("taskkill", "/F", "/IM", p[len(p)-1])
	case "linux", "darwin":
		b, err := ioutil.ReadFile(beego.AppPath + "/" + f + ".pid")
		if err == nil {
			c = exec.Command("/bin/bash", "-c", `kill -9 `+string(b))
		} else {
			log.Println("停止服务失败,pid文件不存在")
		}
	}
	err = c.Run()
	if err != nil {
		log.Println("停止服务失败,", err)
	} else {
		log.Println("停止服务成功")
	}
}
