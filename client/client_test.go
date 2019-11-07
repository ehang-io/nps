package client

import (
	"net"
	"sync"
	"testing"

	"github.com/cnlh/nps/lib/common"
	conn2 "github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/file"
)

func TestConfig(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:8284")
	if err != nil {
		t.Fail()
	}
	c := conn2.NewConn(conn)
	c.SetAlive("tcp")
	if _, err := c.Write([]byte(common.Getverifyval("123"))); err != nil {
		t.Fail()
	}
	c.WriteConfig()
	config := &file.Config{
		U:              "1",
		P:              "2",
		Compress:       "snappy",
		Crypt:          true,
		CompressEncode: 0,
		CompressDecode: 0,
	}
	host := &file.Host{
		Host:         "a.o.com",
		Target:       "127.0.0.1:8080",
		HeaderChange: "",
		HostChange:   "",
		Flow:         nil,
		Client:       nil,
		Remark:       "111",
		NowIndex:     0,
		TargetArr:    nil,
		NoStore:      false,
		RWMutex:      sync.RWMutex{},
	}
	tunnel := &file.Tunnel{
		Port:   9001,
		Mode:   "tcp",
		Target: "127.0.0.1:8082",
		Remark: "333",
	}
	var b []byte
	if b, err = c.ReadLen(16); err != nil {
		t.Fail()
	}
	if _, err := c.SendConfigInfo(config); err != nil {
		t.Fail()
	}
	if !c.GetAddStatus() {
		t.Fail()
	}
	if _, err := c.SendHostInfo(host); err != nil {
		t.Fail()
	}
	if !c.GetAddStatus() {
		t.Fail()
	}
	if _, err := c.SendTaskInfo(tunnel); err != nil {
		t.Fail()
	}
	if !c.GetAddStatus() {
		t.Fail()
	}
	c.Close()
	NewRPClient("127.0.0.1:8284", string(b), "tcp").Start()
}
