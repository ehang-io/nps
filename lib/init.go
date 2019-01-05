package lib

import (
	"errors"
	"flag"
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
	u            = flag.String("u", "", "验证用户名(socks5和web)")
	p            = flag.String("p", "", "验证密码(socks5和web)")
	compress     = flag.String("compress", "", "数据压缩方式（snappy）")
	serverAddr   = flag.String("server", "", "服务器地址ip:端口")
	crypt        = flag.String("crypt", "false", "是否加密(true|false)")
	mux          = flag.String("mux", "false", "是否TCP多路复用(true|false)")
	config       Config
	err          error
	RunList      map[string]interface{} //运行中的任务
	bridge       *Tunnel
	CsvDb        *Csv
)

const cryptKey = "1234567812345678"

func init() {
	RunList = make(map[string]interface{})
}

func InitMode() {
	flag.Parse()
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
		cnf := ServerConfig{
			TcpPort:        *httpPort,
			Mode:           *rpMode,
			Target:         *tunnelTarget,
			VerifyKey:      *verifyKey,
			U:              *u,
			P:              *p,
			Compress:       *compress,
			Start:          0,
			IsRun:          0,
			ClientStatus:   0,
			Crypt:          GetBoolByStr(*crypt),
			Mux:            GetBoolByStr(*mux),
			CompressEncode: 0,
			CompressDecode: 0,
		}
		cnf.CompressDecode, cnf.CompressEncode = getCompressType(cnf.Compress)
		if svr := newMode(bridge, &cnf);
			svr != nil {
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

func newMode(bridge *Tunnel, config *ServerConfig) interface{} {
	switch config.Mode {
	case "httpServer":
		return NewHttpModeServer(bridge, config)
	case "tunnelServer":
		return NewTunnelModeServer(ProcessTunnel, bridge, config)
	case "socks5Server":
		return NewSock5ModeServer(bridge, config)
	case "httpProxyServer":
		return NewTunnelModeServer(ProcessHttp, bridge, config)
	case "udpServer":
		return NewUdpModeServer(bridge, config)
	case "webServer":
		InitCsvDb()
		return NewWebServer(bridge)
	case "hostServer":
		return NewHostServer(config)
	case "httpHostServer":
		return NewTunnelModeServer(ProcessHost, bridge, config)
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

func AddTask(t *ServerConfig) error {
	t.CompressDecode, t.CompressEncode = getCompressType(t.Compress)
	if svr := newMode(bridge, t); svr != nil {
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
