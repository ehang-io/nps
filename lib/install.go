package lib

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func InstallNps() {
	var path string
	switch runtime.GOOS {
	case "windows":
		path = "C:/"
	case "linux", "darwin":
		path = "/etc/nps/"
	}
	if err := os.Mkdir(path, 0755); err != nil {
		log.Fatalf("创建目录%s失败:%s", path, err.Error())
	}
	//复制文件到对应目录
	if err := CopyDir("./web", path); err != nil {
		log.Fatalln(err)
	}
	if err := CopyDir("./conf", path); err != nil {
		log.Fatalln(err)
	}
	//linux加入到/etc/init.d

	//windows处理

	//darwin处理
}

func CopyDir(srcPath string, destPath string) error {
	//检测目录正确性
	if srcInfo, err := os.Stat(srcPath); err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		if !srcInfo.IsDir() {
			e := errors.New("srcPath不是一个正确的目录！")
			fmt.Println(e.Error())
			return e
		}
	}
	if destInfo, err := os.Stat(destPath); err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		if !destInfo.IsDir() {
			e := errors.New("destInfo不是一个正确的目录！")
			fmt.Println(e.Error())
			return e
		}
	}
	//加上拷贝时间:不用可以去掉
	destPath = destPath + "_" + time.Now().Format("20060102150405")

	err := filepath.Walk(srcPath, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if !f.IsDir() {
			path := strings.Replace(path, "\\", "/", -1)
			destNewPath := strings.Replace(path, srcPath, destPath, -1)
			fmt.Println("复制文件:" + path + " 到 " + destNewPath)
			copyFile(path, destNewPath)
		}
		return nil
	})
	if err != nil {
		fmt.Printf(err.Error())
	}
	return err
}

//生成目录并拷贝文件
func copyFile(src, dest string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer srcFile.Close()
	//分割path目录
	destSplitPathDirs := strings.Split(dest, "/")

	//检测时候存在目录
	destSplitPath := ""
	for index, dir := range destSplitPathDirs {
		if index < len(destSplitPathDirs)-1 {
			destSplitPath = destSplitPath + dir + "/"
			b, _ := pathExists(destSplitPath)
			if b == false {
				fmt.Println("创建目录:" + destSplitPath)
				//创建目录
				err := os.Mkdir(destSplitPath, os.ModePerm)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}
	dstFile, err := os.Create(dest)
	if err != nil {
		fmt.Println(err.Error())
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
