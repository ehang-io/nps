package mux

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type connLog struct {
	startTime time.Time
	isClose   bool
	logs      []string
}

var m map[int]*connLog

var copyMap map[int]*connLog
var stashTimeNow time.Time

func deepCopyMap() {
	stashTimeNow = time.Now()
	copyMap = make(map[int]*connLog)
	for k, v := range m {
		copyMap[k] = &connLog{
			startTime: v.startTime,
			isClose:   v.isClose,
			logs:      v.logs,
		}
	}
}

func init() {
	m = make(map[int]*connLog)
	m[0] = &connLog{
		startTime: time.Now(),
		isClose:   false,
		logs:      []string{"111", "222", "333"},
	}
	m[1] = &connLog{
		startTime: time.Now(),
		isClose:   false,
		logs:      []string{"111", "222", "333", "444"},
	}
	m[2] = &connLog{
		startTime: time.Now(),
		isClose:   true,
		logs:      []string{"111", "222", "333", "555"},
	}
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
	deepCopyMap()
	w.Write([]byte("ok"))
}

func index(w http.ResponseWriter, r *http.Request) {
	var keys []int
	for k := range copyMap {
		keys = append(keys, k)
	}
	sort.Sort(IntSlice(keys))
	var s string
	for v := range keys {
		connL := copyMap[v]
		s += "<a href='/detail?id=" + strconv.Itoa(v) + "'>" + strconv.Itoa(v) + "</a>----------"
		s += strconv.Itoa(int(stashTimeNow.Unix()-connL.startTime.Unix())) + "s----------"
		s += strconv.FormatBool(connL.isClose)
		s += "<br>"
	}
	w.Write([]byte(s))
}

func detail(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	i, _ := strconv.Atoi(id)
	v, _ := copyMap[i]
	var s string
	for _, vv := range v.logs {
		s += "<p>" + vv + "</p>"
	}
	w.Write([]byte(s))
}
