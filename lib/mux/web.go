package mux

import (
	"fmt"
	"github.com/astaxie/beego/logs"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type connLog struct {
	startTime time.Time
	isClose   bool
	logs      []string
}

var logms map[int]*connLog
var logmc map[int]*connLog

var copyMaps map[int]*connLog
var copyMapc map[int]*connLog
var stashTimeNow time.Time
var mutex sync.Mutex

func deepCopyMaps() {
	copyMaps = make(map[int]*connLog)
	for k, v := range logms {
		copyMaps[k] = &connLog{
			startTime: v.startTime,
			isClose:   v.isClose,
			logs:      v.logs,
		}
	}
}

func deepCopyMapc() {
	copyMapc = make(map[int]*connLog)
	for k, v := range logmc {
		copyMapc[k] = &connLog{
			startTime: v.startTime,
			isClose:   v.isClose,
			logs:      v.logs,
		}
	}
}

func init() {
	logms = make(map[int]*connLog)
	logmc = make(map[int]*connLog)
}

type IntSlice []int

func (s IntSlice) Len() int { return len(s) }

func (s IntSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s IntSlice) Less(i, j int) bool { return s[i] < s[j] }

func NewLogServer() {
	http.HandleFunc("/", index)
	http.HandleFunc("/detail", detail)
	http.HandleFunc("/stash", stash)
	fmt.Println(http.ListenAndServe(":8899", nil))
}

func stash(w http.ResponseWriter, r *http.Request) {
	stashTimeNow = time.Now()
	deepCopyMaps()
	deepCopyMapc()
	w.Write([]byte("ok"))
}

func getM(label string, id int) (cL *connLog) {
	label = strings.TrimSpace(label)
	mutex.Lock()
	defer mutex.Unlock()
	if label == "nps" {
		cL = logms[id]
	}
	if label == "npc" {
		cL = logmc[id]
	}
	return
}

func setM(label string, id int, cL *connLog) {
	label = strings.TrimSpace(label)
	mutex.Lock()
	defer mutex.Unlock()
	if label == "nps" {
		logms[id] = cL
	}
	if label == "npc" {
		logmc[id] = cL
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	var keys []int
	for k := range copyMaps {
		keys = append(keys, k)
	}
	sort.Sort(IntSlice(keys))
	var s string
	s += "<h1>nps</h1>"
	for _, v := range keys {
		connL := copyMaps[v]
		s += "<a href='/detail?id=" + strconv.Itoa(v) + "&label=nps" + "'>" + strconv.Itoa(v) + "</a>----------"
		s += strconv.Itoa(int(stashTimeNow.Sub(connL.startTime).Milliseconds())) + "ms----------"
		s += strconv.FormatBool(connL.isClose)
		s += "<br>"
	}

	keys = keys[:0]
	s += "<h1>npc</h1>"
	for k := range copyMapc {
		keys = append(keys, k)
	}
	sort.Sort(IntSlice(keys))

	for _, v := range keys {
		connL := copyMapc[v]
		s += "<a href='/detail?id=" + strconv.Itoa(v) + "&label=npc" + "'>" + strconv.Itoa(v) + "</a>----------"
		s += strconv.Itoa(int(stashTimeNow.Sub(connL.startTime).Milliseconds())) + "ms----------"
		s += strconv.FormatBool(connL.isClose)
		s += "<br>"
	}
	w.Write([]byte(s))
}

func detail(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	label := r.FormValue("label")
	logs.Warn(label)
	i, _ := strconv.Atoi(id)
	var v *connLog
	if label == "nps" {
		v, _ = copyMaps[i]
	}
	if label == "npc" {
		v, _ = copyMapc[i]
	}
	var s string
	if v != nil {
		for i, vv := range v.logs {
			s += "<p>" + strconv.Itoa(i+1) + ":" + vv + "</p>"
		}
	}
	w.Write([]byte(s))
}
