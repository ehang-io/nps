package daemon

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ehang.io/nps/lib/common"
)

func InitDaemon(f string, runPath string, pidPath string) {
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
		start(args, f, pidPath, runPath)
		os.Exit(0)
	case "stop":
		stop(f, args[0], pidPath)
		os.Exit(0)
	case "restart":
		stop(f, args[0], pidPath)
		start(args, f, pidPath, runPath)
		os.Exit(0)
	case "status":
		if status(f, pidPath) {
			log.Printf("%s is running", f)
		} else {
			log.Printf("%s is not running", f)
		}
		os.Exit(0)
	case "reload":
		reload(f, pidPath)
		os.Exit(0)
	}
}

func reload(f string, pidPath string) {
	if f == "nps" && !common.IsWindows() && !status(f, pidPath) {
		log.Println("reload fail")
		return
	}
	var c *exec.Cmd
	var err error
	b, err := ioutil.ReadFile(filepath.Join(pidPath, f+".pid"))
	if err == nil {
		c = exec.Command("/bin/bash", "-c", `kill -30 `+string(b))
	} else {
		log.Fatalln("reload error,pid file does not exist")
	}
	if c.Run() == nil {
		log.Println("reload success")
	} else {
		log.Println("reload fail")
	}
}

func status(f string, pidPath string) bool {
	var cmd *exec.Cmd
	b, err := ioutil.ReadFile(filepath.Join(pidPath, f+".pid"))
	if err == nil {
		if !common.IsWindows() {
			cmd = exec.Command("/bin/sh", "-c", "ps -ax | awk '{ print $1 }' | grep "+string(b))
		} else {
			cmd = exec.Command("tasklist")
		}
		out, _ := cmd.Output()
		if strings.Index(string(out), string(b)) > -1 {
			return true
		}
	}
	return false
}

func start(osArgs []string, f string, pidPath, runPath string) {
	if status(f, pidPath) {
		log.Printf(" %s is running", f)
		return
	}
	cmd := exec.Command(osArgs[0], osArgs[1:]...)
	cmd.Start()
	if cmd.Process.Pid > 0 {
		log.Println("start ok , pid:", cmd.Process.Pid, "config path:", runPath)
		d1 := []byte(strconv.Itoa(cmd.Process.Pid))
		ioutil.WriteFile(filepath.Join(pidPath, f+".pid"), d1, 0600)
	} else {
		log.Println("start error")
	}
}

func stop(f string, p string, pidPath string) {
	if !status(f, pidPath) {
		log.Printf(" %s is not running", f)
		return
	}
	var c *exec.Cmd
	var err error
	if common.IsWindows() {
		p := strings.Split(p, `\`)
		c = exec.Command("taskkill", "/F", "/IM", p[len(p)-1])
	} else {
		b, err := ioutil.ReadFile(filepath.Join(pidPath, f+".pid"))
		if err == nil {
			c = exec.Command("/bin/bash", "-c", `kill -9 `+string(b))
		} else {
			log.Fatalln("stop error,pid file does not exist")
		}
	}
	err = c.Run()
	if err != nil {
		log.Println("stop error,", err)
	} else {
		log.Println("stop ok")
	}
}
