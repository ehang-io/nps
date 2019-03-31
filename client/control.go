package client

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/version"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"github.com/cnlh/nps/vender/golang.org/x/net/proxy"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
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
	//read now vKey and write to server
	if f, err := common.ReadAllFromFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt")); err != nil {
		log.Fatalln(err)
	} else if _, err := c.Write([]byte(crypt.Md5(string(f)))); err != nil {
		log.Fatalln(err)
	}
	var isPub bool
	binary.Read(c, binary.LittleEndian, &isPub)
	if l, err := c.GetLen(); err != nil {
		log.Fatalln(err)
	} else if b, err := c.GetShortContent(l); err != nil {
		log.Fatalln(err)
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
			if v.Mode == "secret" {
				ports = append(ports, 0)
			}
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
	if err != nil || cnf.CommonConfig == nil {
		logs.Error("Config file %s loading error %s", path, err.Error())
		os.Exit(0)
	}
	logs.Info("Loading configuration file %s successfully", path)

re:
	if first || cnf.CommonConfig.AutoReconnection {
		if !first {
			logs.Info("Reconnecting...")
			time.Sleep(time.Second * 5)
		}
	} else {
		return
	}
	first = false
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, cnf.CommonConfig.Server, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		logs.Error(err)
		goto re
	}
	var isPub bool
	binary.Read(c, binary.LittleEndian, &isPub)

	// get tmp password
	var b []byte
	vkey := cnf.CommonConfig.VKey
	if isPub {
		// send global configuration to server and get status of config setting
		if _, err := c.SendInfo(cnf.CommonConfig.Client, common.NEW_CONF); err != nil {
			logs.Error(err)
			goto re
		}
		if !c.GetAddStatus() {
			logs.Error("the web_user may have been occupied!")
			goto re
		}

		if b, err = c.GetShortContent(16); err != nil {
			logs.Error(err)
			goto re
		}
		vkey = string(b)
	}
	ioutil.WriteFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt"), []byte(vkey), 0600)

	//send hosts to server
	for _, v := range cnf.Hosts {
		if _, err := c.SendInfo(v, common.NEW_HOST); err != nil {
			logs.Error(err)
			goto re
		}
		if !c.GetAddStatus() {
			logs.Error(errAdd, v.Host)
			goto re
		}
	}

	//send  task to server
	for _, v := range cnf.Tasks {
		if _, err := c.SendInfo(v, common.NEW_TASK); err != nil {
			logs.Error(err)
			goto re
		}
		if !c.GetAddStatus() {
			logs.Error(errAdd, v.Ports, v.Remark)
			goto re
		}
		if v.Mode == "file" {
			//start local file server
			go startLocalFileServer(cnf.CommonConfig, v, vkey)
		}
	}

	//create local server secret or p2p
	for _, v := range cnf.LocalServer {
		go StartLocalServer(v, cnf.CommonConfig)
	}

	c.Close()
	if cnf.CommonConfig.Client.WebUserName == "" || cnf.CommonConfig.Client.WebPassword == "" {
		logs.Notice("web access login username:user password:%s", vkey)
	} else {
		logs.Notice("web access login username:%s password:%s", cnf.CommonConfig.Client.WebUserName, cnf.CommonConfig.Client.WebPassword)
	}
	NewRPClient(cnf.CommonConfig.Server, vkey, cnf.CommonConfig.Tp, cnf.CommonConfig.ProxyUrl, cnf).Start()
	CloseLocalServer()
	goto re
}

// Create a new connection with the server and verify it
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
			switch u.Scheme {
			case "socks5":
				n, er := proxy.FromURL(u, nil)
				if er != nil {
					return nil, er
				}
				connection, err = n.Dial("tcp", server)
			case "http":
				connection, err = NewHttpProxyConn(u, server)
			}
		} else {
			connection, err = net.Dial("tcp", server)
		}
	} else {
		sess, err = kcp.DialWithOptions(server, nil, 10, 3)
		if err == nil {
			conn.SetUdpSession(sess)
			connection = sess
		}
	}
	if err != nil {
		return nil, err
	}
	c := conn.NewConn(connection)
	if _, err := c.Write([]byte(common.CONN_TEST)); err != nil {
		return nil, err
	}
	if _, err := c.Write([]byte(crypt.Md5(version.GetVersion()))); err != nil {
		return nil, err
	}
	if b, err := c.GetShortContent(32); err != nil || crypt.Md5(version.GetVersion()) != string(b) {
		logs.Error("The client does not match the server version. The current version of the client is", version.GetVersion())
		os.Exit(0)
	}
	if _, err := c.Write([]byte(common.Getverifyval(vkey))); err != nil {
		return nil, err
	}
	if s, err := c.ReadFlag(); err != nil {
		return nil, err
	} else if s == common.VERIFY_EER {
		logs.Error("Validation key %s incorrect", vkey)
		os.Exit(0)
	}
	if _, err := c.Write([]byte(connType)); err != nil {
		return nil, err
	}
	c.SetAlive(tp)

	return c, nil
}

func NewHttpProxyConn(url *url.URL, remoteAddr string) (net.Conn, error) {
	req := &http.Request{
		Method: "CONNECT",
		URL:    url,
		Host:   remoteAddr,
		Header: http.Header{},
		Proto:  "HTTP/1.1",
	}
	password, _ := url.User.Password()
	req.Header.Set("Proxy-Authorization", "Basic "+basicAuth(url.User.Username(), password))
	b, err := httputil.DumpRequest(req, false)
	if err != nil {
		return nil, err
	}
	proxyConn, err := net.Dial("tcp", url.Host)
	if err != nil {
		return nil, err
	}
	if _, err := proxyConn.Write(b); err != nil {
		return nil, err
	}
	buf := make([]byte, 1024)
	if _, err := proxyConn.Read(buf); err != nil {
		return nil, err
	}
	return proxyConn, nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
