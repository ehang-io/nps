package main

import (
	"C"
	"ehang.io/nps/client"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego/logs"
	"strconv"
)

var (
	cl *client.TRPClient
	ls bool
)

//export StartClientByVerifyKey
func StartClientByVerifyKey(serverAddr, verifyKey, connType, proxyUrl *C.char) int {
	_ = logs.SetLogger("store")
	if cl != nil {
		cl.Close()
	}
	cl = client.NewRPClient(C.GoString(serverAddr), C.GoString(verifyKey), C.GoString(connType), C.GoString(proxyUrl), nil, 60)
	cl.Start()
	return 1
}

//export StartLocalServer
func StartLocalServer(serverAddr, verifyKey, connType, password, localType, localPortStr, target, proxyUrl *C.char) int {
	ls = true
	_ = logs.SetLogger("store")
	var localPort int
	localPort, _ = strconv.Atoi(C.GoString(localPortStr))
	client.CloseLocalServer()
	commonConfig := new(config.CommonConfig)
	commonConfig.Server = C.GoString(serverAddr)
	commonConfig.VKey = C.GoString(verifyKey)
	commonConfig.Tp = C.GoString(connType)
	commonConfig.ProxyUrl = C.GoString(proxyUrl)
	localServer := new(config.LocalServer)
	localServer.Type = C.GoString(localType)
	localServer.Password = C.GoString(password)
	localServer.Target = C.GoString(target)
	localServer.Port = localPort
	commonConfig.Client = new(file.Client)
	commonConfig.Client.Cnf = new(file.Config)
	client.StartLocalServer(localServer, commonConfig)
	return 1
}

//export GetClientStatus
func GetClientStatus() int {
	if ls && len(client.LocalServer) > 0 {
		return 1
	}
	return client.NowStatus
}

//export CloseClient
func CloseClient() {
	if cl != nil {
		cl.Close()
	}
	if ls {
		client.CloseLocalServer()
		ls = false
	}
}

//export Version
func Version() *C.char {
	return C.CString(version.VERSION)
}

//export Logs
func Logs() *C.char {
	return C.CString(common.GetLogMsg())
}

func main() {
	// Need a main function to make CGO compile package as C shared library
}
