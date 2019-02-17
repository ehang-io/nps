package client

import (
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"github.com/cnlh/nps/vender/golang.org/x/net/proxy"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func GetTaskStatus(path string) {
	cnf, err := config.NewConfig(path)
	if err != nil {
		log.Fatalln(err)
	}
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		log.Fatalln(err)
	}
	if _, err := c.Write([]byte(common.WORK_STATUS)); err != nil {
		log.Fatalln(err)
	}
	if f, err := common.ReadAllFromFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt")); err != nil {
		log.Fatalln(err)
	} else if _, err := c.Write([]byte(string(f))); err != nil {
		log.Fatalln(err)
	}
	if l, err := c.GetLen(); err != nil {
		log.Fatalln(err)
	} else if b, err := c.ReadLen(l); err != nil {
		lg.Fatalln(err)
	} else {
		arr := strings.Split(string(b), common.CONN_DATA_SEQ)
		for _, v := range cnf.Hosts {
			if common.InStrArr(arr, v.Remark) {
				log.Println(v.Remark, "ok")
			} else {
				log.Println(v.Remark, "not running")
			}
		}
		for _, v := range cnf.Tasks {
			ports := common.GetPorts(v.Ports)
			for _, vv := range ports {
				var remark string
				if len(ports) > 1 {
					remark = v.Remark + "_" + strconv.Itoa(vv)
				} else {
					remark = v.Remark
				}
				if common.InStrArr(arr, remark) {
					log.Println(remark, "ok")
				} else {
					log.Println(remark, "not running")
				}
			}
		}
	}
	os.Exit(0)
}

var errAdd = errors.New("The server returned an error, which port or host may have been occupied or not allowed to open.")

func StartFromFile(path string) {
	first := true
	cnf, err := config.NewConfig(path)
	if err != nil {
		lg.Fatalln(err)
	}
	lg.Printf("Loading configuration file %s successfully", path)
re:
	if first || cnf.CommonConfig.AutoReconnection {
		if !first {
			lg.Println("Reconnecting...")
			time.Sleep(time.Second * 5)
		}
	} else {
		return
	}
	first = false
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		lg.Println(err)
		goto re
	}
	if _, err := c.SendConfigInfo(cnf.CommonConfig.Cnf); err != nil {
		lg.Println(err)
		goto re
	}
	var b []byte
	if b, err = c.ReadLen(16); err != nil {
		lg.Println(err)
		goto re
	} else {
		ioutil.WriteFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt"), []byte(string(b)), 0600)
	}
	if !c.GetAddStatus() {
		lg.Println(errAdd)
		goto re
	}
	for _, v := range cnf.Hosts {
		if _, err := c.SendHostInfo(v); err != nil {
			lg.Println(err)
			goto re
		}
		if !c.GetAddStatus() {
			lg.Println(errAdd, v.Host)
			goto re
		}
	}
	for _, v := range cnf.Tasks {
		if _, err := c.SendTaskInfo(v); err != nil {
			lg.Println(err)
			goto re
		}
		if !c.GetAddStatus() {
			lg.Println(errAdd, v.Ports)
			goto re
		}
	}

	c.Close()

	NewRPClient(cnf.CommonConfig.Server, string(b), cnf.CommonConfig.Tp, cnf.CommonConfig.ProxyUrl).Start()
	goto re
}

//Create a new connection with the server and verify it
func NewConn(tp string, vkey string, server string, connType string, proxyUrl string) (*conn.Conn, error) {
	var err error
	var connection net.Conn
	var sess *kcp.UDPSession
	if tp == "tcp" {
		if proxyUrl != "" {
			u, er := url.Parse(proxyUrl)
			if er != nil {
				return nil, er
			}
			n, er := proxy.FromURL(u, nil)
			if er != nil {
				return nil, er
			}
			connection, err = n.Dial("tcp", server)
		} else {
			connection, err = net.Dial("tcp", server)
		}
	} else {
		sess, err = kcp.DialWithOptions(server, nil, 10, 3)
		conn.SetUdpSession(sess)
		connection = sess
	}
	if err != nil {
		return nil, err
	}
	c := conn.NewConn(connection)
	if _, err := c.Write([]byte(common.Getverifyval(vkey))); err != nil {
		lg.Println(err)
	}
	if s, err := c.ReadFlag(); err != nil {
		lg.Println(err)
	} else if s == common.VERIFY_EER {
		lg.Fatalf("Validation key %s incorrect", vkey)
	}
	if _, err := c.Write([]byte(connType)); err != nil {
		lg.Println(err)
	}
	c.SetAlive(tp)

	return c, nil
}
