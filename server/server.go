package server

import (
	"errors"
	"github.com/astaxie/beego"
	"github.com/cnlh/easyProxy/bridge"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"reflect"
	"strings"
	"sync"
)

type RunServer struct {
	flag       int   //标志
	ExportFlow int64 //出口流量
	InletFlow  int64 //入口流量
	service    interface{}
	sync.Mutex
}

var (
	Bridge    *bridge.Tunnel
	RunList   map[int]interface{} //运行中的任务
	CsvDb     = utils.GetCsvDb()
	VerifyKey string
)

func init() {
	RunList = make(map[int]interface{})
}

//从csv文件中恢复任务
func InitFromCsv() {
	for _, v := range CsvDb.Tasks {
		if v.Start == 1 {
			log.Println("启动模式：", v.Mode, "监听端口：", v.TcpPort)
			AddTask(v)
		}
	}
}

//start a new server
func StartNewServer(bridgePort int, cnf *utils.ServerConfig) {
	Bridge = bridge.NewTunnel(bridgePort, RunList)
	if err := Bridge.StartTunnel(); err != nil {
		log.Fatalln("服务端开启失败", err)
	}
	if svr := NewMode(Bridge, cnf); svr != nil {
		RunList[cnf.Id] = svr
		err := reflect.ValueOf(svr).MethodByName("Start").Call(nil)[0]
		if err.Interface() != nil {
			log.Println(err)
		}
	} else {
		log.Fatalln("启动模式不正确")
	}
}

//new a server by mode name
func NewMode(Bridge *bridge.Tunnel, c *utils.ServerConfig) interface{} {
	config := utils.DeepCopyConfig(c)
	switch config.Mode {
	case "tunnelServer":
		return NewTunnelModeServer(ProcessTunnel, Bridge, config)
	case "socks5Server":
		return NewSock5ModeServer(Bridge, config)
	case "httpProxyServer":
		return NewTunnelModeServer(ProcessHttp, Bridge, config)
	case "udpServer":
		return NewUdpModeServer(Bridge, config)
	case "webServer":
		InitFromCsv()
		p, _ := beego.AppConfig.Int("hostPort")
		t := &utils.ServerConfig{
			TcpPort:      p,
			Mode:         "httpHostServer",
			Target:       "",
			U:            "",
			P:            "",
			Compress:     "",
			Start:        1,
			IsRun:        0,
			ClientStatus: 0,
		}
		AddTask(t)
		return NewWebServer(Bridge)
	case "hostServer":
		return NewHostServer(config)
	case "httpHostServer":
		return NewTunnelModeServer(ProcessHost, Bridge, config)
	}
	return nil
}

//stop server
func StopServer(id int) error {
	if v, ok := RunList[id]; ok {
		reflect.ValueOf(v).MethodByName("Close").Call(nil)
		if t, err := CsvDb.GetTask(id); err != nil {
			return err
		} else {
			t.Start = 0
			CsvDb.UpdateTask(t)
		}
		return nil
	}
	return errors.New("未在运行中")
}

//add task
func AddTask(t *utils.ServerConfig) error {
	if svr := NewMode(Bridge, t); svr != nil {
		RunList[t.Id] = svr
		go func() {
			err := reflect.ValueOf(svr).MethodByName("Start").Call(nil)[0]
			if err.Interface() != nil {
				log.Println("客户端", t.Id, "启动失败，错误：", err)
				delete(RunList, t.Id)
			}
		}()
	} else {
		return errors.New("启动模式不正确")
	}
	return nil
}

//start task
func StartTask(id int) error {
	if t, err := CsvDb.GetTask(id); err != nil {
		return err
	} else {
		AddTask(t)
		t.Start = 1
		CsvDb.UpdateTask(t)
	}
	return nil
}

//delete task
func DelTask(id int) error {
	if err := StopServer(id); err != nil {
		return err
	}
	return CsvDb.DelTask(id)
}

//get key by host from x
func GetKeyByHost(host string) (h *utils.HostList, t *utils.Client, err error) {
	for _, v := range CsvDb.Hosts {
		s := strings.Split(host, ":")
		if s[0] == v.Host {
			h = v
			t, err = CsvDb.GetClient(v.ClientId)
			return
		}
	}
	err = errors.New("未找到host对应的内网目标")
	return
}

//get task list by page num
func GetServerConfig(start, length int, typeVal string, clientId int) ([]*utils.ServerConfig, int) {
	list := make([]*utils.ServerConfig, 0)
	var cnt int
	for _, v := range CsvDb.Tasks {
		if (typeVal != "" && v.Mode != typeVal) || (typeVal == "" && clientId != v.ClientId) {
			continue
		}
		if v.UseClientCnf {
			v = utils.DeepCopyConfig(v)
			if c, err := CsvDb.GetClient(v.ClientId); err == nil {
				v.Compress = c.Cnf.Compress
				v.Mux = c.Cnf.Mux
				v.Crypt = c.Cnf.Crypt
				v.U = c.Cnf.U
				v.P = c.Cnf.P
			}
		}
		cnt++
		if start--; start < 0 {
			if length--; length > 0 {
				if _, ok := RunList[v.Id]; ok {
					v.IsRun = 1
				} else {
					v.IsRun = 0
				}
				if s, ok := Bridge.SignalList[v.ClientId]; ok {
					if s.Len() > 0 {
						v.ClientStatus = 1
					} else {
						v.ClientStatus = 0
					}
				} else {
					v.ClientStatus = 0
				}
				list = append(list, v)
			}
		}
	}
	return list, cnt
}

//获取客户端列表
func GetClientList(start, length int) (list []*utils.Client, cnt int) {
	list, cnt = CsvDb.GetClientList(start, length)
	dealClientData(list)
	return
}

func dealClientData(list []*utils.Client) {
	for _, v := range list {
		if _, ok := Bridge.SignalList[v.Id]; ok {
			v.IsConnect = true
		} else {
			v.IsConnect = false
		}
		v.Flow.InletFlow = 0
		v.Flow.ExportFlow = 0
		for _, h := range CsvDb.Hosts {
			if h.ClientId == v.Id {
				v.Flow.InletFlow += h.Flow.InletFlow
				v.Flow.ExportFlow += h.Flow.ExportFlow
			}
		}
		for _, t := range CsvDb.Tasks {
			if t.ClientId == v.Id {
				v.Flow.InletFlow += t.Flow.InletFlow
				v.Flow.ExportFlow += t.Flow.ExportFlow
			}
		}
	}
	return
}

//根据客户端id删除其所属的所有隧道和域名
func DelTunnelAndHostByClientId(clientId int) {
	for _, v := range CsvDb.Tasks {
		if v.ClientId == clientId {
			DelTask(v.Id)
		}
	}
	for _, v := range CsvDb.Hosts {
		if v.ClientId == clientId {
			CsvDb.DelHost(v.Host)
		}
	}
}

//关闭客户端连接
func DelClientConnect(clientId int) {
	Bridge.DelClientTunnel(clientId)
	Bridge.DelClientSignal(clientId)
}

func GetDashboardData() map[string]int {
	data := make(map[string]int)
	data["hostCount"] = len(CsvDb.Hosts)
	data["clientCount"] = len(CsvDb.Clients)
	list := CsvDb.Clients
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
	for _, v := range CsvDb.Tasks {
		switch v.Mode {
		case "tunnelServer":
			data["tunnelServerCount"] += 1
		case "socks5Server":
			data["socks5ServerCount"] += 1
		case "httpProxyServer":
			data["httpProxyServerCount"] += 1
		case "udpServer":
			data["udpServerCount"] += 1
		}
	}
	return data
}
