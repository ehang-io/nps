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
	data, err := http.Get("https://api.github.com/repos/cnlh/nps/releases/latest")
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
