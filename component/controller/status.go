package controller

import (
	"container/list"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"math"
	"net/http"
	"runtime"
	"time"
)

var (
	tData = list.New()
)

type timeData struct {
	now               time.Time
	cpuData           float64
	load1Data         float64
	load5Data         float64
	load15Data        float64
	swapData          float64
	virtualData       float64
	bandwidthRecvData float64
	bandwidthSendData float64
	tcpConnNumData    float64
	udpConnNumData    float64
	diskData          float64
}

type dataAddr []float64

func status(c *gin.Context) {
	timeArr := make([]string, 0)
	dataMap := make(map[string][]float64, 0)
	dataMap["cpu"] = make([]float64, 0)
	dataMap["load1"] = make([]float64, 0)
	dataMap["load5"] = make([]float64, 0)
	dataMap["load15"] = make([]float64, 0)
	dataMap["swap"] = make([]float64, 0)
	dataMap["virtual"] = make([]float64, 0)
	dataMap["bandwidthRecvData"] = make([]float64, 0)
	dataMap["bandwidthSendData"] = make([]float64, 0)
	dataMap["tcpConnNumData"] = make([]float64, 0)
	dataMap["udpConnNumData"] = make([]float64, 0)
	dataMap["disk"] = make([]float64, 0)
	now := tData.Front()
	for {
		if now == nil {
			break
		}
		data := now.Value.(*timeData)
		timeArr = append(timeArr, data.now.Format("01-02 15:04"))
		dataMap["cpu"] = append(dataMap["cpu"], data.cpuData)
		dataMap["load1"] = append(dataMap["load1"], data.load15Data)
		dataMap["load5"] = append(dataMap["load5"], data.load5Data)
		dataMap["load15"] = append(dataMap["load15"], data.load15Data)
		dataMap["swap"] = append(dataMap["swap"], data.swapData)
		dataMap["virtual"] = append(dataMap["virtual"], data.virtualData)
		dataMap["bandwidthSend"] = append(dataMap["bandwidthSend"], data.bandwidthRecvData)
		dataMap["bandwidthRecv"] = append(dataMap["bandwidthRecv"], data.bandwidthSendData)
		dataMap["tcp"] = append(dataMap["v"], data.tcpConnNumData)
		dataMap["udp"] = append(dataMap["udp"], data.udpConnNumData)
		dataMap["disk"] = append(dataMap["disk"], data.diskData)
		now = now.Next()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"result": gin.H{
			"time": timeArr,
			"data": dataMap,
		},
	})
}

func storeSystemStatus() {
	path := "/"
	if runtime.GOOS == "windows" {
		path = "C:"
	}

	for range time.NewTicker(time.Second).C {
		td := &timeData{now: time.Now()}
		checkListLen(tData)
		cpuPercent, err := cpu.Percent(0, true)
		if err == nil {
			var cpuAll float64
			for _, v := range cpuPercent {
				cpuAll += v
			}
			td.cpuData = float64(len(cpuPercent))
		}

		loads, err := load.Avg()
		if err == nil {
			td.load1Data = loads.Load1
			td.load1Data = loads.Load5
			td.load15Data = loads.Load15
		}

		swap, err := mem.SwapMemory()
		if err == nil {
			td.swapData = math.Round(swap.UsedPercent)
		}
		vir, err := mem.VirtualMemory()
		if err == nil {
			td.virtualData = math.Round(vir.UsedPercent)
		}
		io1, err := net.IOCounters(false)
		if err == nil {
			time.Sleep(time.Millisecond * 500)
			io2, err := net.IOCounters(false)
			if err == nil && len(io2) > 0 && len(io1) > 0 {
				td.bandwidthRecvData = float64((io2[0].BytesRecv-io1[0].BytesRecv)*2) / 1024 / 1024
				td.bandwidthSendData = float64((io2[0].BytesSent-io1[0].BytesSent)*2) / 1024 / 1024
			}
		}
		conn, err := net.ProtoCounters(nil)
		if err == nil {
			for _, v := range conn {
				if v.Protocol == "tcp" {
					td.tcpConnNumData = float64(v.Stats["CurrEstab"])
				}
				if v.Protocol == "udp" {
					td.udpConnNumData = float64(v.Stats["CurrEstab"])
				}
			}
		}
		usage, err := disk.Usage(path)
		if err == nil {
			td.diskData = math.Round(usage.UsedPercent)
		}
		tData.PushBack(td)
	}
}

func checkListLen(lists ...*list.List) {
	for _, l := range lists {
		if l.Len() > 4320 {
			if first := l.Front(); first != nil {
				l.Remove(first)
			}
		}
	}
}
