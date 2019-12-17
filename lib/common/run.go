package common

import (
	"os"
	"path/filepath"
	"runtime"
)

//Get the currently selected configuration file directory
//For non-Windows systems, select the /etc/nps as config directory if exist, or select ./
//windows system, select the C:\Program Files\nps as config directory if exist, or select ./
func GetRunPath() string {
	var path string
	if path = GetInstallPath(); !FileExists(path) {
		return GetAppPath()
	}
	return path
}

//Different systems get different installation paths
func GetInstallPath() string {
	var path string
	if IsWindows() {
		path = `C:\Program Files\nps`
	} else {
		path = "/etc/nps"
	}
	return path
}

//Get the absolute path to the running directory
func GetAppPath() string {
	if path, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		return path
	}
	return os.Args[0]
}

//Determine whether the current system is a Windows system?
func IsWindows() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return false
}

//interface log file path
func GetLogPath() string {
	var path string
	if IsWindows() {
		path = filepath.Join(GetAppPath(), "nps.log")
	} else {
		path = "/var/log/nps.log"
	}
	return path
}

//interface npc log file path
func GetNpcLogPath() string {
	var path string
	if IsWindows() {
		path = filepath.Join(GetAppPath(), "npc.log")
	} else {
		path = "/var/log/npc.log"
	}
	return path
}

//interface pid file path
func GetTmpPath() string {
	var path string
	if IsWindows() {
		path = GetAppPath()
	} else {
		path = "/tmp"
	}
	return path
}
