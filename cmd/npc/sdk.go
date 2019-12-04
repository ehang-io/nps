package main

import "C"
import (
	"github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/client"
	"time"
)

func init() {
	logs.SetLogger(logs.AdapterFile, `{"filename":"npc.log","daily":false,"maxlines":100000,"color":true}`)
}

var status int
var closeBefore int
var cl *client.TRPClient

//export StartClientByVerifyKey
func StartClientByVerifyKey(serverAddr, verifyKey, connType, proxyUrl *C.char) int {
	if cl != nil {
		closeBefore = 1
		cl.Close()
	}
	cl = client.NewRPClient(C.GoString(serverAddr), C.GoString(verifyKey), C.GoString(connType), C.GoString(proxyUrl), nil)
	closeBefore = 0
	go func() {
		for {
			status = 1
			cl.Start()
			status = 0
			if closeBefore == 1 {
				return
			}
			time.Sleep(time.Second * 5)
		}
	}()
	return 1
}

//export GetClientStatus
func GetClientStatus() int {
	return status
}

//export CloseClient
func CloseClient() {
	closeBefore = 1
	cl.Close()
}

func main() {
	// Need a main function to make CGO compile package as C shared library
}
