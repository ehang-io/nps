package install

import (
	"ehang.io/nps/lib/common"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/c4milo/unpackit"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Keep it in sync with the template from service_sysv_linux.go file
// Use "ps | grep -v grep | grep $(get_pid)" because "ps PID" may not work on OpenWrt
const SysvScript = `#!/bin/sh
# For RedHat and cousins:
# chkconfig: - 99 01
# description: {{.Description}}
# processname: {{.Path}}
### BEGIN INIT INFO
# Provides:          {{.Path}}
# Required-Start:
# Required-Stop:
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: {{.DisplayName}}
# Description:       {{.Description}}
### END INIT INFO
cmd="{{.Path}}{{range .Arguments}} {{.|cmd}}{{end}}"
name=$(basename $(readlink -f $0))
pid_file="/var/run/$name.pid"
stdout_log="/var/log/$name.log"
stderr_log="/var/log/$name.err"
[ -e /etc/sysconfig/$name ] && . /etc/sysconfig/$name
get_pid() {
    cat "$pid_file"
}
is_running() {
    [ -f "$pid_file" ] && ps | grep -v grep | grep $(get_pid) > /dev/null 2>&1
}
case "$1" in
    start)
        if is_running; then
            echo "Already started"
        else
            echo "Starting $name"
            {{if .WorkingDirectory}}cd '{{.WorkingDirectory}}'{{end}}
            $cmd >> "$stdout_log" 2>> "$stderr_log" &
            echo $! > "$pid_file"
            if ! is_running; then
                echo "Unable to start, see $stdout_log and $stderr_log"
                exit 1
            fi
        fi
    ;;
    stop)
        if is_running; then
            echo -n "Stopping $name.."
            kill $(get_pid)
            for i in $(seq 1 10)
            do
                if ! is_running; then
                    break
                fi
                echo -n "."
                sleep 1
            done
            echo
            if is_running; then
                echo "Not stopped; may still be shutting down or shutdown may have failed"
                exit 1
            else
                echo "Stopped"
                if [ -f "$pid_file" ]; then
                    rm "$pid_file"
                fi
            fi
        else
            echo "Not running"
        fi
    ;;
    restart)
        $0 stop
        if is_running; then
            echo "Unable to stop, will not attempt to start"
            exit 1
        fi
        $0 start
    ;;
    status)
        if is_running; then
            echo "Running"
        else
            echo "Stopped"
            exit 1
        fi
    ;;
    *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
exit 0
`

const SystemdScript = `[Unit]
Description={{.Description}}
ConditionFileIsExecutable={{.Path|cmdEscape}}
{{range $i, $dep := .Dependencies}} 
{{$dep}} {{end}}
[Service]
LimitNOFILE=65536
StartLimitInterval=5
StartLimitBurst=10
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmd}}{{end}}
{{if .ChRoot}}RootDirectory={{.ChRoot|cmd}}{{end}}
{{if .WorkingDirectory}}WorkingDirectory={{.WorkingDirectory|cmdEscape}}{{end}}
{{if .UserName}}User={{.UserName}}{{end}}
{{if .ReloadSignal}}ExecReload=/bin/kill -{{.ReloadSignal}} "$MAINPID"{{end}}
{{if .PIDFile}}PIDFile={{.PIDFile|cmd}}{{end}}
{{if and .LogOutput .HasOutputFileSupport -}}
StandardOutput=file:/var/log/{{.Name}}.out
StandardError=file:/var/log/{{.Name}}.err
{{- end}}
Restart=always
RestartSec=120
[Install]
WantedBy=multi-user.target
`

func UpdateNps() {
	destPath := downloadLatest("server")
	//复制文件到对应目录
	copyStaticFile(destPath, "nps")
	fmt.Println("Update completed, please restart")
}

func UpdateNpc() {
	destPath := downloadLatest("client")
	//复制文件到对应目录
	copyStaticFile(destPath, "npc")
	fmt.Println("Update completed, please restart")
}

type release struct {
	TagName string `json:"tag_name"`
}

func downloadLatest(bin string) string {
	// get version
	data, err := http.Get("https://api.github.com/repos/ehang-io/nps/releases/latest")
	if err != nil {
		log.Fatal(err.Error())
	}
	b, err := ioutil.ReadAll(data.Body)
	if err != nil {
		log.Fatal(err)
	}
	rl := new(release)
	json.Unmarshal(b, &rl)
	version := rl.TagName
	fmt.Println("the latest version is", version)
	filename := runtime.GOOS + "_" + runtime.GOARCH + "_" + bin + ".tar.gz"
	// download latest package
	downloadUrl := fmt.Sprintf("https://ehang.io/nps/releases/download/%s/%s", version, filename)
	fmt.Println("download package from ", downloadUrl)
	resp, err := http.Get(downloadUrl)
	if err != nil {
		log.Fatal(err.Error())
	}
	destPath, err := unpackit.Unpack(resp.Body, "")
	if err != nil {
		log.Fatal(err)
	}
	if bin == "server" {
		destPath = strings.Replace(destPath, "/web", "", -1)
		destPath = strings.Replace(destPath, `\web`, "", -1)
		destPath = strings.Replace(destPath, "/views", "", -1)
		destPath = strings.Replace(destPath, `\views`, "", -1)
	} else {
		destPath = strings.Replace(destPath, `\conf`, "", -1)
		destPath = strings.Replace(destPath, "/conf", "", -1)
	}
	return destPath
}

func copyStaticFile(srcPath, bin string) string {
	path := common.GetInstallPath()
	if bin == "nps" {
		//复制文件到对应目录
		if err := CopyDir(filepath.Join(srcPath, "web", "views"), filepath.Join(path, "web", "views")); err != nil {
			log.Fatalln(err)
		}
		chMod(filepath.Join(path, "web", "views"), 0766)
		if err := CopyDir(filepath.Join(srcPath, "web", "static"), filepath.Join(path, "web", "static")); err != nil {
			log.Fatalln(err)
		}
		chMod(filepath.Join(path, "web", "static"), 0766)
	}
	binPath, _ := filepath.Abs(os.Args[0])
	if !common.IsWindows() {
		if _, err := copyFile(filepath.Join(srcPath, bin), "/usr/bin/"+bin); err != nil {
			if _, err := copyFile(filepath.Join(srcPath, bin), "/usr/local/bin/"+bin); err != nil {
				log.Fatalln(err)
			} else {
				copyFile(filepath.Join(srcPath, bin), "/usr/local/bin/"+bin+"-update")
				chMod("/usr/local/bin/"+bin+"-update", 0755)
				binPath = "/usr/local/bin/" + bin
			}
		} else {
			copyFile(filepath.Join(srcPath, bin), "/usr/bin/"+bin+"-update")
			chMod("/usr/bin/"+bin+"-update", 0755)
			binPath = "/usr/bin/" + bin
		}
	} else {
		copyFile(filepath.Join(srcPath, bin+".exe"), filepath.Join(common.GetAppPath(), bin+"-update.exe"))
		copyFile(filepath.Join(srcPath, bin+".exe"), filepath.Join(common.GetAppPath(), bin+".exe"))
	}
	chMod(binPath, 0755)
	return binPath
}

func InstallNpc() {
	path := common.GetInstallPath()
	if !common.FileExists(path) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
	copyStaticFile(common.GetAppPath(), "npc")
}

func InstallNps() string {
	path := common.GetInstallPath()
	if common.FileExists(path) {
		MkidrDirAll(path, "web/static", "web/views")
	} else {
		MkidrDirAll(path, "conf", "web/static", "web/views")
		// not copy config if the config file is exist
		if err := CopyDir(filepath.Join(common.GetAppPath(), "conf"), filepath.Join(path, "conf")); err != nil {
			log.Fatalln(err)
		}
		chMod(filepath.Join(path, "conf"), 0766)
	}
	binPath := copyStaticFile(common.GetAppPath(), "nps")
	log.Println("install ok!")
	log.Println("Static files and configuration files in the current directory will be useless")
	log.Println("The new configuration file is located in", path, "you can edit them")
	if !common.IsWindows() {
		log.Println(`You can start with:
nps start|stop|restart|uninstall|update or nps-update update
anywhere!`)
	} else {
		log.Println(`You can copy executable files to any directory and start working with:
nps.exe start|stop|restart|uninstall|update or nps-update.exe update
now!`)
	}
	chMod(common.GetLogPath(), 0777)
	return binPath
}
func MkidrDirAll(path string, v ...string) {
	for _, item := range v {
		if err := os.MkdirAll(filepath.Join(path, item), 0755); err != nil {
			log.Fatalf("Failed to create directory %s error:%s", path, err.Error())
		}
	}
}

func CopyDir(srcPath string, destPath string) error {
	//检测目录正确性
	if srcInfo, err := os.Stat(srcPath); err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		if !srcInfo.IsDir() {
			e := errors.New("SrcPath is not the right directory!")
			return e
		}
	}
	if destInfo, err := os.Stat(destPath); err != nil {
		return err
	} else {
		if !destInfo.IsDir() {
			e := errors.New("DestInfo is not the right directory!")
			return e
		}
	}
	err := filepath.Walk(srcPath, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if !f.IsDir() {
			destNewPath := strings.Replace(path, srcPath, destPath, -1)
			log.Println("copy file ::" + path + " to " + destNewPath)
			copyFile(path, destNewPath)
			if !common.IsWindows() {
				chMod(destNewPath, 0766)
			}
		}
		return nil
	})
	return err
}

//生成目录并拷贝文件
func copyFile(src, dest string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()
	//分割path目录
	destSplitPathDirs := strings.Split(dest, string(filepath.Separator))

	//检测时候存在目录
	destSplitPath := ""
	for index, dir := range destSplitPathDirs {
		if index < len(destSplitPathDirs)-1 {
			destSplitPath = destSplitPath + dir + string(filepath.Separator)
			b, _ := pathExists(destSplitPath)
			if b == false {
				log.Println("mkdir:" + destSplitPath)
				//创建目录
				err := os.Mkdir(destSplitPath, os.ModePerm)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
	}
	dstFile, err := os.Create(dest)
	if err != nil {
		return
	}
	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

//检测文件夹路径时候存在
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func chMod(name string, mode os.FileMode) {
	if !common.IsWindows() {
		os.Chmod(name, mode)
	}
}
