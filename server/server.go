package server

import (
	"ehang.io/nps/lib/version"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/proxy"
	"ehang.io/nps/server/tool"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

var (
	Bridge  *bridge.Bridge
	RunList sync.Map //map[int]interface{}
)

func init() {
	RunList = sync.Map{}
}

//init task from db
func InitFromCsv() {
	//Add a public password
	if vkey := beego.AppConfig.String("public_vkey"); vkey != "" {
		c := file.NewClient(vkey, true, true)
		file.GetDb().NewClient(c)
		RunList.Store(c.Id, nil)
		//RunList[c.Id] = nil
	}
	//Initialize services in server-side files
	file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		if value.(*file.Tunnel).Status {
			AddTask(value.(*file.Tunnel))
		}
		return true
	})
}

//get bridge command
func DealBridgeTask() {
	for {
		select {
		case t := <-Bridge.OpenTask:
			AddTask(t)
		case t := <-Bridge.CloseTask:
			StopServer(t.Id)
		case id := <-Bridge.CloseClient:
			DelTunnelAndHostByClientId(id, true)
			if v, ok := file.GetDb().JsonDb.Clients.Load(id); ok {
				if v.(*file.Client).NoStore {
					file.GetDb().DelClient(id)
				}
			}
		case tunnel := <-Bridge.OpenTask:
			StartTask(tunnel.Id)
		case s := <-Bridge.SecretChan:
			logs.Trace("New secret connection, addr", s.Conn.Conn.RemoteAddr())
			if t := file.GetDb().GetTaskByMd5Password(s.Password); t != nil {
				if t.Status {
					go proxy.NewBaseServer(Bridge, t).DealClient(s.Conn, t.Client, t.Target.TargetStr, nil, common.CONN_TCP, nil, t.Flow, t.Target.LocalProxy)
				} else {
					s.Conn.Close()
					logs.Trace("This key %s cannot be processed,status is close", s.Password)
				}
			} else {
				logs.Trace("This key %s cannot be processed", s.Password)
				s.Conn.Close()
			}
		}
	}
}

//start a new server
func StartNewServer(bridgePort int, cnf *file.Tunnel, bridgeType string, bridgeDisconnect int) {
	Bridge = bridge.NewTunnel(bridgePort, bridgeType, common.GetBoolByStr(beego.AppConfig.String("ip_limit")), RunList, bridgeDisconnect)
	go func() {
		if err := Bridge.StartTunnel(); err != nil {
			logs.Error("start server bridge error", err)
			os.Exit(0)
		}
	}()
	if p, err := beego.AppConfig.Int("p2p_port"); err == nil {
		go proxy.NewP2PServer(p).Start()
		go proxy.NewP2PServer(p + 1).Start()
		go proxy.NewP2PServer(p + 2).Start()
	}
	go DealBridgeTask()
	go dealClientFlow()
	if svr := NewMode(Bridge, cnf); svr != nil {
		if err := svr.Start(); err != nil {
			logs.Error(err)
		}
		RunList.Store(cnf.Id, svr)
		//RunList[cnf.Id] = svr
	} else {
		logs.Error("Incorrect startup mode %s", cnf.Mode)
	}
}

func dealClientFlow() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dealClientData()
		}
	}
}

//new a server by mode name
func NewMode(Bridge *bridge.Bridge, c *file.Tunnel) proxy.Service {
	var service proxy.Service
	switch c.Mode {
	case "tcp", "file":
		service = proxy.NewTunnelModeServer(proxy.ProcessTunnel, Bridge, c)
	case "socks5":
		service = proxy.NewSock5ModeServer(Bridge, c)
	case "httpProxy":
		service = proxy.NewTunnelModeServer(proxy.ProcessHttp, Bridge, c)
	case "tcpTrans":
		service = proxy.NewTunnelModeServer(proxy.HandleTrans, Bridge, c)
	case "udp":
		service = proxy.NewUdpModeServer(Bridge, c)
	case "webServer":
		InitFromCsv()
		t := &file.Tunnel{
			Port:   0,
			Mode:   "httpHostServer",
			Status: true,
		}
		AddTask(t)
		service = proxy.NewWebServer(Bridge)
	case "httpHostServer":
		httpPort, _ := beego.AppConfig.Int("http_proxy_port")
		httpsPort, _ := beego.AppConfig.Int("https_proxy_port")
		useCache, _ := beego.AppConfig.Bool("http_cache")
		cacheLen, _ := beego.AppConfig.Int("http_cache_length")
		addOrigin, _ := beego.AppConfig.Bool("http_add_origin_header")
		service = proxy.NewHttp(Bridge, c, httpPort, httpsPort, useCache, cacheLen, addOrigin)
	}
	return service
}

//stop server
func StopServer(id int) error {
	//if v, ok := RunList[id]; ok {
	if v, ok := RunList.Load(id); ok {
		if svr, ok := v.(proxy.Service); ok {
			if err := svr.Close(); err != nil {
				return err
			}
			logs.Info("stop server id %d", id)
		} else {
			logs.Warn("stop server id %d error", id)
		}
		if t, err := file.GetDb().GetTask(id); err != nil {
			return err
		} else {
			t.Status = false
			file.GetDb().UpdateTask(t)
		}
		//delete(RunList, id)
		RunList.Delete(id)
		return nil
	}
	return errors.New("task is not running")
}

//add task
func AddTask(t *file.Tunnel) error {
	if t.Mode == "secret" || t.Mode == "p2p" {
		logs.Info("secret task %s start ", t.Remark)
		//RunList[t.Id] = nil
		RunList.Store(t.Id, nil)
		return nil
	}
	if b := tool.TestServerPort(t.Port, t.Mode); !b && t.Mode != "httpHostServer" {
		logs.Error("taskId %d start error port %d open failed", t.Id, t.Port)
		return errors.New("the port open error")
	}
	if minute, err := beego.AppConfig.Int("flow_store_interval"); err == nil && minute > 0 {
		go flowSession(time.Minute * time.Duration(minute))
	}
	if svr := NewMode(Bridge, t); svr != nil {
		logs.Info("tunnel task %s start mode：%s port %d", t.Remark, t.Mode, t.Port)
		//RunList[t.Id] = svr
		RunList.Store(t.Id, svr)
		go func() {
			if err := svr.Start(); err != nil {
				logs.Error("clientId %d taskId %d start error %s", t.Client.Id, t.Id, err)
				//delete(RunList, t.Id)
				RunList.Delete(t.Id)
				return
			}
		}()
	} else {
		return errors.New("the mode is not correct")
	}
	return nil
}

//start task
func StartTask(id int) error {
	if t, err := file.GetDb().GetTask(id); err != nil {
		return err
	} else {
		AddTask(t)
		t.Status = true
		file.GetDb().UpdateTask(t)
	}
	return nil
}

//delete task
func DelTask(id int) error {
	//if _, ok := RunList[id]; ok {
	if _, ok := RunList.Load(id); ok {
		if err := StopServer(id); err != nil {
			return err
		}
	}
	return file.GetDb().DelTask(id)
}

//get task list by page num
func GetTunnel(start, length int, typeVal string, clientId int, search string) ([]*file.Tunnel, int) {
	list := make([]*file.Tunnel, 0)
	var cnt int
	keys := file.GetMapKeys(file.GetDb().JsonDb.Tasks, false, "", "")
	for _, key := range keys {
		if value, ok := file.GetDb().JsonDb.Tasks.Load(key); ok {
			v := value.(*file.Tunnel)
			if (typeVal != "" && v.Mode != typeVal || (clientId != 0 && v.Client.Id != clientId)) || (typeVal == "" && clientId != v.Client.Id) {
				continue
			}
			if search != "" && !(v.Id == common.GetIntNoErrByStr(search) || v.Port == common.GetIntNoErrByStr(search) || strings.Contains(v.Password, search) || strings.Contains(v.Remark, search)) {
				continue
			}
			cnt++
			if _, ok := Bridge.Client.Load(v.Client.Id); ok {
				v.Client.IsConnect = true
			} else {
				v.Client.IsConnect = false
			}
			if start--; start < 0 {
				if length--; length >= 0 {
					//if _, ok := RunList[v.Id]; ok {
					if _, ok := RunList.Load(v.Id); ok {
						v.RunStatus = true
					} else {
						v.RunStatus = false
					}
					list = append(list, v)
				}
			}
		}
	}
	return list, cnt
}

//get client list
func GetClientList(start, length int, search, sort, order string, clientId int) (list []*file.Client, cnt int) {
	list, cnt = file.GetDb().GetClientList(start, length, search, sort, order, clientId)
	dealClientData()
	return
}

func dealClientData() {
	file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
		v := value.(*file.Client)
		if vv, ok := Bridge.Client.Load(v.Id); ok {
			v.IsConnect = true
			v.Version = vv.(*bridge.Client).Version
		} else {
			v.IsConnect = false
		}
		v.Flow.InletFlow = 0
		v.Flow.ExportFlow = 0
		file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
			h := value.(*file.Host)
			if h.Client.Id == v.Id {
				v.Flow.InletFlow += h.Flow.InletFlow
				v.Flow.ExportFlow += h.Flow.ExportFlow
			}
			return true
		})
		file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
			t := value.(*file.Tunnel)
			if t.Client.Id == v.Id {
				v.Flow.InletFlow += t.Flow.InletFlow
				v.Flow.ExportFlow += t.Flow.ExportFlow
			}
			return true
		})
		return true
	})
	return
}

//delete all host and tasks by client id
func DelTunnelAndHostByClientId(clientId int, justDelNoStore bool) {
	var ids []int
	file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*file.Tunnel)
		if justDelNoStore && !v.NoStore {
			return true
		}
		if v.Client.Id == clientId {
			ids = append(ids, v.Id)
		}
		return true
	})
	for _, id := range ids {
		DelTask(id)
	}
	ids = ids[:0]
	file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*file.Host)
		if justDelNoStore && !v.NoStore {
			return true
		}
		if v.Client.Id == clientId {
			ids = append(ids, v.Id)
		}
		return true
	})
	for _, id := range ids {
		file.GetDb().DelHost(id)
	}
}

//close the client
func DelClientConnect(clientId int) {
	Bridge.DelClient(clientId)
}

func GetDashboardData() map[string]interface{} {
	data := make(map[string]interface{})
	data["version"] = version.VERSION
	data["hostCount"] = common.GeSynctMapLen(file.GetDb().JsonDb.Hosts)
	data["clientCount"] = common.GeSynctMapLen(file.GetDb().JsonDb.Clients)
	if beego.AppConfig.String("public_vkey") != "" { //remove public vkey
		data["clientCount"] = data["clientCount"].(int) - 1
	}
	dealClientData()
	c := 0
	var in, out int64
	file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
		v := value.(*file.Client)
		if v.IsConnect {
			c += 1
		}
		in += v.Flow.InletFlow
		out += v.Flow.ExportFlow
		return true
	})
	data["clientOnlineCount"] = c
	data["inletFlowCount"] = int(in)
	data["exportFlowCount"] = int(out)
	var tcp, udp, secret, socks5, p2p, http int
	file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		switch value.(*file.Tunnel).Mode {
		case "tcp":
			tcp += 1
		case "socks5":
			socks5 += 1
		case "httpProxy":
			http += 1
		case "udp":
			udp += 1
		case "p2p":
			p2p += 1
		case "secret":
			secret += 1
		}
		return true
	})

	data["tcpC"] = tcp
	data["udpCount"] = udp
	data["socks5Count"] = socks5
	data["httpProxyCount"] = http
	data["secretCount"] = secret
	data["p2pCount"] = p2p
	data["bridgeType"] = beego.AppConfig.String("bridge_type")
	data["httpProxyPort"] = beego.AppConfig.String("http_proxy_port")
	data["httpsProxyPort"] = beego.AppConfig.String("https_proxy_port")
	data["ipLimit"] = beego.AppConfig.String("ip_limit")
	data["flowStoreInterval"] = beego.AppConfig.String("flow_store_interval")
	data["serverIp"] = beego.AppConfig.String("p2p_ip")
	data["p2pPort"] = beego.AppConfig.String("p2p_port")
	data["logLevel"] = beego.AppConfig.String("log_level")
	tcpCount := 0

	file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
		tcpCount += int(value.(*file.Client).NowConn)
		return true
	})
	data["tcpCount"] = tcpCount
	cpuPercet, _ := cpu.Percent(0, true)
	var cpuAll float64
	for _, v := range cpuPercet {
		cpuAll += v
	}
	loads, _ := load.Avg()
	data["load"] = loads.String()
	data["cpu"] = math.Round(cpuAll / float64(len(cpuPercet)))
	swap, _ := mem.SwapMemory()
	data["swap_mem"] = math.Round(swap.UsedPercent)
	vir, _ := mem.VirtualMemory()
	data["virtual_mem"] = math.Round(vir.UsedPercent)
	conn, _ := net.ProtoCounters(nil)
	io1, _ := net.IOCounters(false)
	time.Sleep(time.Millisecond * 500)
	io2, _ := net.IOCounters(false)
	if len(io2) > 0 && len(io1) > 0 {
		data["io_send"] = (io2[0].BytesSent - io1[0].BytesSent) * 2
		data["io_recv"] = (io2[0].BytesRecv - io1[0].BytesRecv) * 2
	}
	for _, v := range conn {
		data[v.Protocol] = v.Stats["CurrEstab"]
	}
	//chart
	var fg int
	if len(tool.ServerStatus) >= 10 {
		fg = len(tool.ServerStatus) / 10
		for i := 0; i <= 9; i++ {
			data["sys"+strconv.Itoa(i+1)] = tool.ServerStatus[i*fg]
		}
	}
	return data
}

func flowSession(m time.Duration) {
	ticker := time.NewTicker(m)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			file.GetDb().JsonDb.StoreHostToJsonFile()
			file.GetDb().JsonDb.StoreTasksToJsonFile()
			file.GetDb().JsonDb.StoreClientsToJsonFile()
		}
	}
}
