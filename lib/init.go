package lib

import (
	"errors"
	"flag"
	"github.com/astaxie/beego"
	"log"
	"reflect"
	"strings"
	"sync"
)

var (
	configPath   = flag.String("config", "config.json", "配置文件路径")
	TcpPort      = flag.Int("tcpport", 8284, "客户端与服务端通信端口")
	httpPort     = flag.Int("httpport", 8024, "对外监听的端口")
	rpMode       = flag.String("mode", "client", "启动模式")
	tunnelTarget = flag.String("target", "10.1.50.203:80", "远程目标")
	verifyKey    = flag.String("vkey", "", "验证密钥")
	u            = flag.String("u", "", "socks5验证用户名")
	p            = flag.String("p", "", "socks5验证密码")
	compress     = flag.String("compress", "", "数据压缩方式（gzip|snappy）")
	serverAddr   = flag.String("server", "", "服务器地址ip:端口")
	config       Config
	err          error
	RunList      map[string]interface{} //运行中的任务
	bridge       *Tunnel
	CsvDb        *Csv
)

func init() {
	RunList = make(map[string]interface{})
}

func InitMode() {
	flag.Parse()
	de, en := getCompressType(*compress)
	if *rpMode == "client" {
		JsonParse := NewJsonStruct()
		if config, err = JsonParse.Load(*configPath); err != nil {
			log.Println("配置文件加载失败")
		}
		stop := make(chan int)
		for _, v := range strings.Split(*verifyKey, ",") {
			log.Println("客户端启动，连接：", *serverAddr, " 验证令牌：", v)
			go NewRPClient(*serverAddr, 3, v).Start()
		}
		<-stop
	} else {
		bridge = newTunnel(*TcpPort)
		if err := bridge.StartTunnel(); err != nil {
			log.Fatalln("服务端开启失败", err)
		}
		log.Println("服务端启动，监听tcp服务端端口：", *TcpPort)
		if svr := newMode(*rpMode, bridge, *httpPort, *tunnelTarget, *u, *p, en, de, *verifyKey); svr != nil {
			reflect.ValueOf(svr).MethodByName("Start").Call(nil)
		} else {
			log.Fatalln("启动模式不正确")
		}
	}
}

//从csv文件中恢复任务
func InitFromCsv() {
	for _, v := range CsvDb.Tasks {
		if v.Start == 1 {
			log.Println(""+
				"启动模式：", v.Mode, "监听端口：", v.TcpPort, "客户端令牌：", v.VerifyKey)
			AddTask(v)
		}
	}
}

func newMode(mode string, bridge *Tunnel, httpPort int, tunnelTarget string, u string, p string, enCompress int, deCompress int, vkey string) interface{} {
	if u == "" || p == "" { //如果web管理中设置了用户名和密码，则覆盖配置文件
		u = beego.AppConfig.String("auth.user")
		p = beego.AppConfig.String("auth.password")
	}
	switch mode {
	case "httpServer":
		return NewHttpModeServer(httpPort, bridge, enCompress, deCompress, vkey)
	case "tunnelServer":
		return NewTunnelModeServer(httpPort, tunnelTarget, ProcessTunnel, bridge, enCompress, deCompress, vkey, u, p)
	case "sock5Server":
		return NewSock5ModeServer(httpPort, u, p, bridge, enCompress, deCompress, vkey)
	case "httpProxyServer":
		return NewTunnelModeServer(httpPort, tunnelTarget, ProcessHttp, bridge, enCompress, deCompress, vkey, u, p)
	case "udpServer":
		return NewUdpModeServer(httpPort, tunnelTarget, bridge, enCompress, deCompress, vkey)
	case "webServer":
		InitCsvDb()
		return NewWebServer(bridge)
	case "hostServer":
		return NewHostServer()
	case "httpHostServer":
		return NewTunnelModeServer(httpPort, tunnelTarget, ProcessHost, bridge, enCompress, deCompress, vkey, u, p)
	}
	return nil
}

func StopServer(cFlag string) error {
	if v, ok := RunList[cFlag]; ok {
		reflect.ValueOf(v).MethodByName("Close").Call(nil)
		delete(RunList, cFlag)
		if *verifyKey == "" { //多客户端模式关闭相关隧道
			bridge.DelClientSignal(cFlag)
			bridge.DelClientTunnel(cFlag)
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

func AddTask(t *TaskList) error {
	de, en := getCompressType(t.Compress)
	if svr := newMode(t.Mode, bridge, t.TcpPort, t.Target, t.U, t.P, en, de, t.VerifyKey); svr != nil {
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

func DelTask(vKey string) error {
	if err := StopServer(vKey); err != nil {
		return err
	}
	return CsvDb.DelTask(vKey)
}

func InitCsvDb() *Csv {
	var once sync.Once
	once.Do(func() {
		CsvDb = NewCsv("./conf/", bridge, RunList)
		CsvDb.Init()
	})
	return CsvDb
}
