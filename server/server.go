package server

import (
	"errors"
	"github.com/cnlh/nps/bridge"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/server/proxy"
	"github.com/cnlh/nps/server/tool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"os"
	"time"
)

var (
	Bridge  *bridge.Bridge
	RunList map[int]interface{} //运行中的任务
)

func init() {
	RunList = make(map[int]interface{})
}

//从csv文件中恢复任务
func InitFromCsv() {
	//Add a public password
	if vkey := beego.AppConfig.String("publicVkey"); vkey != "" {
		c := file.NewClient(vkey, true, true)
		file.GetCsvDb().NewClient(c)
		RunList[c.Id] = nil
	}
	//Initialize services in server-side files
	for _, v := range file.GetCsvDb().Tasks {
		if v.Status {
			AddTask(v)
		}
	}
}

func DealBridgeTask() {
	for {
		select {
		case t := <-Bridge.OpenTask:
			AddTask(t)
		case id := <-Bridge.CloseClient:
			DelTunnelAndHostByClientId(id)
			file.GetCsvDb().DelClient(id)
		case s := <-Bridge.SecretChan:
			logs.Trace("New secret connection, addr", s.Conn.Conn.RemoteAddr())
			if t := file.GetCsvDb().GetTaskByMd5Password(s.Password); t != nil {
				if !t.Client.GetConn() {
					logs.Info("Connections exceed the current client %d limit", t.Client.Id)
					s.Conn.Close()
				} else if t.Status {
					go proxy.NewBaseServer(Bridge, t).DealClient(s.Conn, t.Target, nil)
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
func StartNewServer(bridgePort int, cnf *file.Tunnel, bridgeType string) {
	Bridge = bridge.NewTunnel(bridgePort, bridgeType, common.GetBoolByStr(beego.AppConfig.String("ipLimit")), RunList)
	if err := Bridge.StartTunnel(); err != nil {
		logs.Error("服务端开启失败", err)
		os.Exit(0)
	} else {
		logs.Info("Server startup, the bridge type is %s, the bridge port is %d", bridgeType, bridgePort)
	}
	if p, err := beego.AppConfig.Int("p2pPort"); err == nil {
		go proxy.NewP2PServer(p).Start()
	}
	go DealBridgeTask()
	if svr := NewMode(Bridge, cnf); svr != nil {
		if err := svr.Start(); err != nil {
			logs.Error(err)
		}
		RunList[cnf.Id] = svr
	} else {
		logs.Error("Incorrect startup mode %s", cnf.Mode)
	}
}

//new a server by mode name
func NewMode(Bridge *bridge.Bridge, c *file.Tunnel) proxy.Service {
	var service proxy.Service
	switch c.Mode {
	case "tcpServer":
		service = proxy.NewTunnelModeServer(proxy.ProcessTunnel, Bridge, c)
	case "socks5Server":
		service = proxy.NewSock5ModeServer(Bridge, c)
	case "httpProxyServer":
		service = proxy.NewTunnelModeServer(proxy.ProcessHttp, Bridge, c)
	case "udpServer":
		service = proxy.NewUdpModeServer(Bridge, c)
	case "webServer":
		InitFromCsv()
		t := &file.Tunnel{
			Port:   0,
			Mode:   "httpHostServer",
			Target: "",
			Status: true,
		}
		AddTask(t)
		service = proxy.NewWebServer(Bridge)
	case "httpHostServer":
		service = proxy.NewHttp(Bridge, c)
	}
	return service
}

//stop server
func StopServer(id int) error {
	if v, ok := RunList[id]; ok {
		if svr, ok := v.(proxy.Service); ok {
			if err := svr.Close(); err != nil {
				return err
			}
		}
		if t, err := file.GetCsvDb().GetTask(id); err != nil {
			return err
		} else {
			t.Status = false
			file.GetCsvDb().UpdateTask(t)
		}
		delete(RunList, id)
		return nil
	}
	return errors.New("未在运行中")
}

//add task
func AddTask(t *file.Tunnel) error {
	if t.Mode == "secretServer" {
		logs.Info("secret task %s start ", t.Remark)
		RunList[t.Id] = nil
		return nil
	}
	if b := tool.TestServerPort(t.Port, t.Mode); !b && t.Mode != "httpHostServer" {
		logs.Error("taskId %d start error port %d open failed", t.Id, t.Port)
		return errors.New("the port open error")
	}
	if minute, err := beego.AppConfig.Int("flowStoreInterval"); err == nil && minute > 0 {
		go flowSession(time.Minute * time.Duration(minute))
	}
	if svr := NewMode(Bridge, t); svr != nil {
		logs.Info("tunnel task %s start mode：%s port %d", t.Remark, t.Mode, t.Port)
		RunList[t.Id] = svr
		go func() {
			if err := svr.Start(); err != nil {
				logs.Error("clientId %d taskId %d start error %s", t.Client.Id, t.Id, err)
				delete(RunList, t.Id)
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
	if t, err := file.GetCsvDb().GetTask(id); err != nil {
		return err
	} else {
		AddTask(t)
		t.Status = true
		file.GetCsvDb().UpdateTask(t)
	}
	return nil
}

//delete task
func DelTask(id int) error {
	if _, ok := RunList[id]; ok {
		if err := StopServer(id); err != nil {
			return err
		}
	}
	return file.GetCsvDb().DelTask(id)
}

//get task list by page num
func GetTunnel(start, length int, typeVal string, clientId int) ([]*file.Tunnel, int) {
	list := make([]*file.Tunnel, 0)
	var cnt int
	for _, v := range file.GetCsvDb().Tasks {
		if (typeVal != "" && v.Mode != typeVal) || (typeVal == "" && clientId != v.Client.Id) {
			continue
		}
		cnt++
		if _, ok := Bridge.Client[v.Client.Id]; ok {
			v.Client.IsConnect = true
		} else {
			v.Client.IsConnect = false
		}
		if start--; start < 0 {
			if length--; length > 0 {
				if _, ok := RunList[v.Id]; ok {
					v.RunStatus = true
				} else {
					v.RunStatus = false
				}
				list = append(list, v)
			}
		}
	}
	return list, cnt
}

//获取客户端列表
func GetClientList(start, length int) (list []*file.Client, cnt int) {
	list, cnt = file.GetCsvDb().GetClientList(start, length)
	dealClientData(list)
	return
}

func dealClientData(list []*file.Client) {
	for _, v := range list {
		if _, ok := Bridge.Client[v.Id]; ok {
			v.IsConnect = true
		} else {
			v.IsConnect = false
		}
		v.Flow.InletFlow = 0
		v.Flow.ExportFlow = 0
		for _, h := range file.GetCsvDb().Hosts {
			if h.Client.Id == v.Id {
				v.Flow.InletFlow += h.Flow.InletFlow
				v.Flow.ExportFlow += h.Flow.ExportFlow
			}
		}
		for _, t := range file.GetCsvDb().Tasks {
			if t.Client.Id == v.Id {
				v.Flow.InletFlow += t.Flow.InletFlow
				v.Flow.ExportFlow += t.Flow.ExportFlow
			}
		}
	}
	return
}

//根据客户端id删除其所属的所有隧道和域名
func DelTunnelAndHostByClientId(clientId int) {
	var ids []int
	for _, v := range file.GetCsvDb().Tasks {
		if v.Client.Id == clientId {
			ids = append(ids, v.Id)
		}
	}
	for _, id := range ids {
		DelTask(id)
	}
	for _, v := range file.GetCsvDb().Hosts {
		if v.Client.Id == clientId {
			file.GetCsvDb().DelHost(v.Id)
		}
	}
}

//关闭客户端连接
func DelClientConnect(clientId int) {
	Bridge.DelClient(clientId)
}

func GetDashboardData() map[string]int {
	data := make(map[string]int)
	data["hostCount"] = len(file.GetCsvDb().Hosts)
	data["clientCount"] = len(file.GetCsvDb().Clients) - 1 //Remove the public key client
	list := file.GetCsvDb().Clients
	dealClientData(list)
	c := 0
	var in, out int64
	for _, v := range list {
		if v.IsConnect {
			c += 1
		}
		in += v.Flow.InletFlow
		out += v.Flow.ExportFlow
	}
	data["clientOnlineCount"] = c
	data["inletFlowCount"] = int(in)
	data["exportFlowCount"] = int(out)
	for _, v := range file.GetCsvDb().Tasks {
		switch v.Mode {
		case "tcpServer":
			data["tcpServerCount"] += 1
		case "socks5Server":
			data["socks5ServerCount"] += 1
		case "httpProxyServer":
			data["httpProxyServerCount"] += 1
		case "udpServer":
			data["udpServerCount"] += 1
		}
	}
	tcpCount := 0
	for _, v := range file.GetCsvDb().Clients {
		tcpCount += v.NowConn
	}
	data["tcpCount"] = tcpCount
	return data
}

func flowSession(m time.Duration) {
	ticker := time.NewTicker(m)
	for {
		select {
		case <-ticker.C:
			file.GetCsvDb().StoreHostToCsv()
			file.GetCsvDb().StoreTasksToCsv()
		}
	}
}
