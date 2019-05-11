// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// fileLogWriter implements LoggerInterface.
// It writes messages by lines limit, file size limit, or time frequency.
type fileLogWriter struct {
	sync.RWMutex // write log order by order and  atomic incr maxLinesCurLines and maxSizeCurSize
	// The opened file
	Filename   string `json:"filename"`
	fileWriter *os.File

	// Rotate at line
	MaxLines         int `json:"maxlines"`
	maxLinesCurLines int

	MaxFiles         int `json:"maxfiles"`
	MaxFilesCurFiles int

	// Rotate at size
	MaxSize        int `json:"maxsize"`
	maxSizeCurSize int

	// Rotate daily
	Daily         bool  `json:"daily"`
	MaxDays       int64 `json:"maxdays"`
	dailyOpenDate int
	dailyOpenTime time.Time

	// Rotate hourly
	Hourly         bool  `json:"hourly"`
	MaxHours       int64 `json:"maxhours"`
	hourlyOpenDate int
	hourlyOpenTime time.Time

	Rotate bool `json:"rotate"`

	Level int `json:"level"`

	Perm string `json:"perm"`

	RotatePerm string `json:"rotateperm"`

	fileNameOnly, suffix string // like "project.log", project is fileNameOnly and .log is suffix
}

// newFileWriter create a FileLogWriter returning as LoggerInterface.
func newFileWriter() Logger {
	w := &fileLogWriter{
		Daily:      true,
		MaxDays:    7,
		Hourly:     false,
		MaxHours:   168,
		Rotate:     true,
		RotatePerm: "0440",
		Level:      LevelTrace,
		Perm:       "0660",
		MaxLines:   10000000,
		MaxFiles:   999,
		MaxSize:    1 << 28,
	}
	return w
}

// Init file logger with json config.
// jsonConfig like:
//  {
//  "filename":"logs/beego.log",
//  "maxLines":10000,
//  "maxsize":1024,
//  "daily":true,
//  "maxDays":15,
//  "rotate":true,
//      "perm":"0600"
//  }
func (w *fileLogWriter) Init(jsonConfig string) error {
	err := json.Unmarshal([]byte(jsonConfig), w)
	if err != nil {
		return err
	}
	if len(w.Filename) == 0 {
		return errors.New("jsonconfig must have filename")
	}
	w.suffix = filepath.Ext(w.Filename)
	w.fileNameOnly = strings.TrimSuffix(w.Filename, w.suffix)
	if w.suffix == "" {
		w.suffix = ".log"
	}
	err = w.startLogger()
	return err
}

// start file logger. create log file and set to locker-inside file writer.
func (w *fileLogWriter) startLogger() error {
	file, err := w.createLogFile()
	if err != nil {
		return err
	}
	if w.fileWriter != nil {
		w.fileWriter.Close()
	}
	w.fileWriter = file
	return w.initFd()
}

func (w *fileLogWriter) needRotateDaily(size int, day int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize >= w.MaxSize) ||
		(w.Daily && day != w.dailyOpenDate)
}

func (w *fileLogWriter) needRotateHourly(size int, hour int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize >= w.MaxSize) ||
		(w.Hourly && hour != w.hourlyOpenDate)

}

// WriteMsg write logger message into file.
func (w *fileLogWriter) WriteMsg(when time.Time, msg string, level int) error {
	if level > w.Level {
		return nil
	}
	hd, d, h := formatTimeHeader(when)
	msg = string(hd) + msg + "\n"
	if w.Rotate {
		w.RLock()
		if w.needRotateHourly(len(msg), h) {
			w.RUnlock()
			w.Lock()
			if w.needRotateHourly(len(msg), h) {
				if err := w.doRotate(when); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
				}
			}
			w.Unlock()
		} else if w.needRotateDaily(len(msg), d) {
			w.RUnlock()
			w.Lock()
			if w.needRotateDaily(len(msg), d) {
				if err := w.doRotate(when); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
				}
			}
			w.Unlock()
		} else {
			w.RUnlock()
		}
	}

	w.Lock()
	_, err := w.fileWriter.Write([]byte(msg))
	if err == nil {
		w.maxLinesCurLines++
		w.maxSizeCurSize += len(msg)
	}
	w.Unlock()
	return err
}

func (w *fileLogWriter) createLogFile() (*os.File, error) {
	// Open the log file
	perm, err := strconv.ParseInt(w.Perm, 8, 64)
	if err != nil {
		return nil, err
	}

	filepath := path.Dir(w.Filename)
	os.MkdirAll(filepath, os.FileMode(perm))

	fd, err := os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.FileMode(perm))
	if err == nil {
		// Make sure file perm is user set perm cause of `os.OpenFile` will obey umask
		os.Chmod(w.Filename, os.FileMode(perm))
	}
	return fd, err
}

func (w *fileLogWriter) initFd() error {
	fd := w.fileWriter
	fInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat err: %s", err)
	}
	w.maxSizeCurSize = int(fInfo.Size())
	w.dailyOpenTime = time.Now()
	w.dailyOpenDate = w.dailyOpenTime.Day()
	w.hourlyOpenTime = time.Now()
	w.hourlyOpenDate = w.hourlyOpenTime.Hour()
	w.maxLinesCurLines = 0
	if w.Hourly {
		go w.hourlyRotate(w.hourlyOpenTime)
	} else if w.Daily {
		go w.dailyRotate(w.dailyOpenTime)
	}
	if fInfo.Size() > 0 && w.MaxLines > 0 {
		count, err := w.lines()
		if err != nil {
			return err
		}
		w.maxLinesCurLines = count
	}
	return nil
}

func (w *fileLogWriter) dailyRotate(openTime time.Time) {
	y, m, d := openTime.Add(24 * time.Hour).Date()
	nextDay := time.Date(y, m, d, 0, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextDay.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C
	w.Lock()
	if w.needRotateDaily(0, time.Now().Day()) {
		if err := w.doRotate(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

func (w *fileLogWriter) hourlyRotate(openTime time.Time) {
	y, m, d := openTime.Add(1 * time.Hour).Date()
	h, _, _ := openTime.Add(1 * time.Hour).Clock()
	nextHour := time.Date(y, m, d, h, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextHour.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C
	w.Lock()
	if w.needRotateHourly(0, time.Now().Hour()) {
		if err := w.doRotate(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

func (w *fileLogWriter) lines() (int, error) {
	fd, err := os.Open(w.Filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	buf := make([]byte, 32768) // 32k
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}

		count += bytes.Count(buf[:c], lineSep)

		if err == io.EOF {
			break
		}
	}

	return count, nil
}

// DoRotate means it need to write file in new file.
// new file name like xx.2013-01-01.log (daily) or xx.001.log (by line or size)
func (w *fileLogWriter) doRotate(logTime time.Time) error {
	// file exists
	// Find the next available number
	num := w.MaxFilesCurFiles + 1
	fName := ""
	format := ""
	var openTime time.Time
	rotatePerm, err := strconv.ParseInt(w.RotatePerm, 8, 64)
	if err != nil {
		return err
	}

	_, err = os.Lstat(w.Filename)
	if err != nil {
		//even if the file is not exist or other ,we should RESTART the logger
		goto RESTART_LOGGER
	}

	if w.Hourly {
		format = "2006010215"
		openTime = w.hourlyOpenTime
	} else if w.Daily {
		format = "2006-01-02"
		openTime = w.dailyOpenTime
	}

	// only when one of them be setted, then the file would be splited
	if w.MaxLines > 0 || w.MaxSize > 0 {
		for ; err == nil && num <= w.MaxFiles; num++ {
			fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", logTime.Format(format), num, w.suffix)
			_, err = os.Lstat(fName)
		}
	} else {
		fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", openTime.Format(format), num, w.suffix)
		_, err = os.Lstat(fName)
		w.MaxFilesCurFiles = num
	}

	// return error if the last file checked still existed
	if err == nil {
		return fmt.Errorf("Rotate: Cannot find free log number to rename %s", w.Filename)
	}

	// close fileWriter before rename
	w.fileWriter.Close()

	// Rename the file to its new found name
	// even if occurs error,we MUST guarantee to  restart new logger
	err = os.Rename(w.Filename, fName)
	if err != nil {
		goto RESTART_LOGGER
	}

	err = os.Chmod(fName, os.FileMode(rotatePerm))

RESTART_LOGGER:

	startLoggerErr := w.startLogger()
	go w.deleteOldLog()

	if startLoggerErr != nil {
		return fmt.Errorf("Rotate StartLogger: %s", startLoggerErr)
	}
	if err != nil {
		return fmt.Errorf("Rotate: %s", err)
	}
	return nil
}

func (w *fileLogWriter) deleteOldLog() {
	dir := filepath.Dir(w.Filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Unable to delete old log '%s', error: %v\n", path, r)
			}
		}()

		if info == nil {
			return
		}
        if w.Hourly {
            if !info.IsDir() && info.ModTime().Add(1 * time.Hour * time.Duration(w.MaxHours)).Before(time.Now()) {
                if strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
                strings.HasSuffix(filepath.Base(path), w.suffix) {
                    os.Remove(path)
                }
            }
        } else if w.Daily {
            if !info.IsDir() && info.ModTime().Add(24 * time.Hour * time.Duration(w.MaxDays)).Before(time.Now()) {
                if strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
                strings.HasSuffix(filepath.Base(path), w.suffix) {
                    os.Remove(path)
                }
            }
        }
		return
	})
}

// Destroy close the file description, close file writer.
func (w *fileLogWriter) Destroy() {
	w.fileWriter.Close()
}

// Flush flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (w *fileLogWriter) Flush() {
	w.fileWriter.Sync()
}

func init() {
	Register(AdapterFile, newFileWriter)
}
