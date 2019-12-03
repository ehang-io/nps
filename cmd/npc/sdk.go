package main

import "C"
import (
	"fmt"
	"github.com/cnlh/nps/client"
	"time"
)

//export PrintBye
func PrintBye() {
	fmt.Println("From DLL: Bye!")
}

var status bool

//export Sum
func StartClientByVerifyKey(a int, b int) bool {
	c := client.NewRPClient(*serverAddr, *verifyKey, *connType, *proxyUrl, nil)
	go func() {
		for {
			status = true
			c.Start()
			status = false
			time.Sleep(time.Second * 5)
		}
	}()
	return true
}

func main() {
	// Need a main function to make CGO compile package as C shared library
}
