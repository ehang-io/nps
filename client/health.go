package client

import (
	"container/heap"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/sheap"
	"net"
	"net/http"
	"strings"
	"time"
)

func heathCheck(cnf *config.Config, c net.Conn) {
	var hosts []*file.Host
	var tunnels []*file.Tunnel
	h := &sheap.IntHeap{}
	for _, v := range cnf.Hosts {
		if v.HealthMaxFail > 0 && v.HealthCheckTimeout > 0 && v.HealthCheckInterval > 0 {
			v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval))
			heap.Push(h, v.HealthNextTime.Unix())
			v.HealthMap = make(map[string]int)
			hosts = append(hosts, v)
		}
	}
	for _, v := range cnf.Tasks {
		if v.HealthMaxFail > 0 && v.HealthCheckTimeout > 0 && v.HealthCheckInterval > 0 {
			v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval))
			heap.Push(h, v.HealthNextTime.Unix())
			v.HealthMap = make(map[string]int)
			tunnels = append(tunnels, v)
		}
	}
	if len(hosts) == 0 && len(tunnels) == 0 {
		return
	}
	for {
		rs := heap.Pop(h).(int64) - time.Now().Unix()
		if rs < 0 {
			continue
		}
		timer := time.NewTicker(time.Duration(rs))
		select {
		case <-timer.C:
			for _, v := range hosts {
				if v.HealthNextTime.Before(time.Now()) {
					v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval))
					//check
					go checkHttp(v, c)
					//reset time
					heap.Push(h, v.HealthNextTime.Unix())
				}
			}
			for _, v := range tunnels {
				if v.HealthNextTime.Before(time.Now()) {
					v.HealthNextTime = time.Now().Add(time.Duration(v.HealthCheckInterval))
					//check
					go checkTcp(v, c)
					//reset time
					heap.Push(h, v.HealthNextTime.Unix())
				}
			}
		}
	}
}

func checkTcp(t *file.Tunnel, c net.Conn) {
	arr := strings.Split(t.Target, "\n")
	for _, v := range arr {
		if _, err := net.DialTimeout("tcp", v, time.Duration(t.HealthCheckTimeout)); err != nil {
			t.HealthMap[v] += 1
		}
		if t.HealthMap[v] > t.HealthMaxFail {
			t.HealthMap[v] += 1
			if t.HealthMap[v] == t.HealthMaxFail {
				//send fail remove
				ch <- file.NewHealthInfo("tcp", v, true)
			}
		} else if t.HealthMap[v] >= t.HealthMaxFail {
			//send recovery add
			ch <- file.NewHealthInfo("tcp", v, false)
			t.HealthMap[v] = 0
		}
	}
}

func checkHttp(h *file.Host, ch chan *file.HealthInfo) {
	arr := strings.Split(h.Target, "\n")
	client := &http.Client{}
	client.Timeout = time.Duration(h.HealthCheckTimeout) * time.Second
	for _, v := range arr {
		if _, err := client.Get(v + h.HttpHealthUrl); err != nil {
			h.HealthMap[v] += 1
			if h.HealthMap[v] == h.HealthMaxFail {
				//send fail remove
				ch <- file.NewHealthInfo("http", v, true)
			}
		} else if h.HealthMap[v] >= h.HealthMaxFail {
			//send recovery add
			h.HealthMap[v] = 0
			ch <- file.NewHealthInfo("http", v, false)
		}
	}
}
