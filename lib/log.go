package lib

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var Log *log.Logger

func InitLogFile(f string, isStdout bool) {
	var prefix string
	if !isStdout {
		logFile, err := os.OpenFile(filepath.Join(GetLogPath(), f+"_log.txt"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
		if err != nil {
			log.Fatalln("open file error !", err)
		}
		if runtime.GOOS == "windows" {
			prefix = "\r\n"
		}
		Log = log.New(logFile, prefix, log.Ldate|log.Ltime)
	} else {
		Log = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	}
}

func Println(v ...interface{}) {
	Log.Println(v ...)
}

func Fatalln(v ...interface{}) {
	Log.SetPrefix("error ")
	Log.Fatalln(v ...)
	Log.SetPrefix("")
}
func Fatalf(format string, v ...interface{}) {
	Log.SetPrefix("error ")
	Log.Fatalf(format, v...)
	Log.SetPrefix("")
}

func Printf(format string, v ...interface{}) {
	Log.Printf(format, v...)
}
