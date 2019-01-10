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

var (
	Bridge    *bridge.Tunnel
	RunList   map[string]interface{} //运行中的任务
	CsvDb     *Csv
	VerifyKey string
)

func init() {
	RunList = make(map[string]interface{})
}

//从csv文件中恢复任务
func InitFromCsv() {
	for _, v := range CsvDb.Tasks {
		if v.Start == 1 {
			log.Println("启动模式：", v.Mode, "监听端口：", v.TcpPort, "客户端令牌：", v.VerifyKey)
			AddTask(v)
		}
	}
}

//start a new server
func StartNewServer(bridgePort int, cnf *ServerConfig) {
	Bridge = bridge.NewTunnel(bridgePort, RunList)
	if err := Bridge.StartTunnel(); err != nil {
		log.Fatalln("服务端开启失败", err)
	}
	if svr := NewMode(Bridge, cnf); svr != nil {
		RunList[cnf.VerifyKey] = svr
		err := reflect.ValueOf(svr).MethodByName("Start").Call(nil)[0]
		if err.Interface() != nil {
			log.Println(err)
		}
	} else {
		log.Fatalln("启动模式不正确")
	}
}

//new a server by mode name
func NewMode(Bridge *bridge.Tunnel, config *ServerConfig) interface{} {
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
		InitCsvDb()
		InitFromCsv()
		p, _ := beego.AppConfig.Int("hostPort")
		t := &ServerConfig{
			TcpPort:      p,
			Mode:         "httpHostServer",
			Target:       "",
			VerifyKey:    "",
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
func StopServer(cFlag string) error {
	if v, ok := RunList[cFlag]; ok {
		reflect.ValueOf(v).MethodByName("Close").Call(nil)
		delete(RunList, cFlag)
		if VerifyKey == "" { //多客户端模式关闭相关隧道
			Bridge.DelClientSignal(cFlag)
			Bridge.DelClientTunnel(cFlag)
		}
		if t, err := CsvDb.GetTask(cFlag); err != nil {
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
func AddTask(t *ServerConfig) error {
	t.CompressDecode, t.CompressEncode = utils.GetCompressType(t.Compress)
	if svr := NewMode(Bridge, t); svr != nil {
		RunList[t.VerifyKey] = svr
		go func() {
			err := reflect.ValueOf(svr).MethodByName("Start").Call(nil)[0]
			if err.Interface() != nil {
				log.Println("客户端", t.VerifyKey, "启动失败，错误：", err)
				delete(RunList, t.VerifyKey)
			}
		}()
	} else {
		return errors.New("启动模式不正确")
	}
	return nil
}

//start task
func StartTask(vKey string) error {
	if t, err := CsvDb.GetTask(vKey); err != nil {
		return err
	} else {
		AddTask(t)
		t.Start = 1
		CsvDb.UpdateTask(t)
	}
	return nil
}

//delete task
func DelTask(vKey string) error {
	if err := StopServer(vKey); err != nil {
		return err
	}
	return CsvDb.DelTask(vKey)
}

//init csv from file
func InitCsvDb() *Csv {
	var once sync.Once
	once.Do(func() {
		CsvDb = NewCsv(RunList)
		CsvDb.Init()
	})
	return CsvDb
}

//get key by host from x
func GetKeyByHost(host string) (h *HostList, t *ServerConfig, err error) {
	for _, v := range CsvDb.Hosts {
		if strings.Contains(host, v.Host) {
			h = v
			t, err = CsvDb.GetTask(v.Vkey)
			return
		}
	}
	err = errors.New("未找到host对应的内网目标")
	return
}

//get task list by page num
func GetServerConfig(start, length int, typeVal string) ([]*ServerConfig, int) {
	list := make([]*ServerConfig, 0)
	var cnt int
	for _, v := range CsvDb.Tasks {
		if v.Mode != typeVal {
			continue
		}
		cnt++
		if start--; start < 0 {
			if length--; length > 0 {
				if _, ok := RunList[v.VerifyKey]; ok {
					v.IsRun = 1
				} else {
					v.IsRun = 0
				}
				if s, ok := Bridge.SignalList[getverifyval(v.VerifyKey)]; ok {
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

//get verify value
//when mode is webServer and vKey is not none
func getverifyval(vkey string) string {
	if VerifyKey != "" {
		return utils.Md5(VerifyKey)
	}
	return utils.Md5(vkey)
}
