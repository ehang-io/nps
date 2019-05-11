package install

import (
	"errors"
	"fmt"
	"github.com/cnlh/nps/lib/common"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func InstallNps() {
	path := common.GetInstallPath()
	if common.FileExists(path) {
		log.Fatalf("the path %s has exist, does not support install", path)
	}
	MkidrDirAll(path, "conf", "web/static", "web/views")
	//复制文件到对应目录
	if err := CopyDir(filepath.Join(common.GetAppPath(), "web", "views"), filepath.Join(path, "web", "views")); err != nil {
		log.Fatalln(err)
	}
	if err := CopyDir(filepath.Join(common.GetAppPath(), "web", "static"), filepath.Join(path, "web", "static")); err != nil {
		log.Fatalln(err)
	}
	if err := CopyDir(filepath.Join(common.GetAppPath(), "conf"), filepath.Join(path, "conf")); err != nil {
		log.Fatalln(err)
	}

	if !common.IsWindows() {
		if _, err := copyFile(filepath.Join(common.GetAppPath(), "nps"), "/usr/bin/nps"); err != nil {
			if _, err := copyFile(filepath.Join(common.GetAppPath(), "nps"), "/usr/local/bin/nps"); err != nil {
				log.Fatalln(err)
			} else {
				os.Chmod("/usr/local/bin/nps", 0777)
				log.Println("Executable files have been copied to", "/usr/local/bin/nps")
			}
		} else {
			os.Chmod("/usr/bin/nps", 0777)
			log.Println("Executable files have been copied to", "/usr/bin/nps")
		}

	}
	log.Println("install ok!")
	log.Println("Static files and configuration files in the current directory will be useless")
	log.Println("The new configuration file is located in", path, "you can edit them")
	if !common.IsWindows() {
		log.Println("You can start with nps test|start|stop|restart|status anywhere")
	} else {
		log.Println("You can copy executable files to any directory and start working with nps.exe test|start|stop|restart|status")
	}
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
