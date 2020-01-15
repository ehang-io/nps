package client

import (
	"container/heap"
	"net"
	"net/http"
	"strings"
	"time"

	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/sheap"
	"github.com/astaxie/beego/logs"
	"github.com/pkg/errors"
)

var isStart bool
var serverConn *conn.Conn

func heathCheck(healths []*file.Health, c *conn.Conn) bool {
	serverConn = c
	if isStart {
		for _, v := range healths {
			v.HealthMap = make(map[string]int)
		}
		return true
	}
	isStart = true
	h := &sheap.IntHeap{}
	for _, v := range healths {
		if v.HealthMaxFail > 0 && v.HealthCheckTimeout > 0 && v.HealthCheckInterval > 0 {
			v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval) * time.Second)
			heap.Push(h, v.HealthNextTime.Unix())
			v.HealthMap = make(map[string]int)
		}
	}
	go session(healths, h)
	return true
}

func session(healths []*file.Health, h *sheap.IntHeap) {
	for {
		if h.Len() == 0 {
			logs.Error("health check error")
			break
		}
		rs := heap.Pop(h).(int64) - time.Now().Unix()
		if rs <= 0 {
			continue
		}
		timer := time.NewTimer(time.Duration(rs) * time.Second)
		select {
		case <-timer.C:
			for _, v := range healths {
				if v.HealthNextTime.Before(time.Now()) {
					v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval) * time.Second)
					//check
					go check(v)
					//reset time
					heap.Push(h, v.HealthNextTime.Unix())
				}
			}
		}
	}
}

// work when just one port and many target
func check(t *file.Health) {
	arr := strings.Split(t.HealthCheckTarget, ",")
	var err error
	var rs *http.Response
	for _, v := range arr {
		if t.HealthCheckType == "tcp" {
			var c net.Conn
			c, err = net.DialTimeout("tcp", v, time.Duration(t.HealthCheckTimeout)*time.Second)
			if err == nil {
				c.Close()
			}
		} else {
			client := &http.Client{}
			client.Timeout = time.Duration(t.HealthCheckTimeout) * time.Second
			rs, err = client.Get("http://" + v + t.HttpHealthUrl)
			if err == nil && rs.StatusCode != 200 {
				err = errors.New("status code is not match")
			}
		}
		t.Lock()
		if err != nil {
			t.HealthMap[v] += 1
		} else if t.HealthMap[v] >= t.HealthMaxFail {
			//send recovery add
			serverConn.SendHealthInfo(v, "1")
			t.HealthMap[v] = 0
		}

		if t.HealthMap[v] > 0 && t.HealthMap[v]%t.HealthMaxFail == 0 {
			//send fail remove
			serverConn.SendHealthInfo(v, "0")
		}
		t.Unlock()
	}
}
