package main

import "C"
import (
	"github.com/cnlh/nps/client"
	"time"
)

var status bool
var closeBefore bool
var cl *client.TRPClient

//export StartClientByVerifyKey
func StartClientByVerifyKey(serverAddr, verifyKey, connType, proxyUrl string) bool {
	if cl != nil {
		closeBefore = true
		cl.Close()
	}
	cl = client.NewRPClient(serverAddr, verifyKey, connType, proxyUrl, nil)
	closeBefore = false
	go func() {
		for {
			status = true
			cl.Start()
			status = false
			if closeBefore {
				return
			}
			time.Sleep(time.Second * 5)
		}
	}()
	return true
}

//export GetClientStatus
func GetClientStatus() bool {
	return status
}

//export CloseClient
func CloseClient() {
	cl.Close()
	closeBefore = true
}

func main() {
	// Need a main function to make CGO compile package as C shared library
}
