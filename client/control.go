package client

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego/logs"
	"github.com/xtaci/kcp-go"
	"golang.org/x/net/proxy"
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
	NewRPClient(cnf.CommonConfig.Server, vkey, cnf.CommonConfig.Tp, cnf.CommonConfig.ProxyUrl, cnf, cnf.CommonConfig.DisconnectTime).Start()
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
			default:
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
	connection.SetDeadline(time.Now().Add(time.Second * 10))
	defer connection.SetDeadline(time.Time{})
	c := conn.NewConn(connection)
	if _, err := c.Write([]byte(common.CONN_TEST)); err != nil {
		return nil, err
	}
	if err := c.WriteLenContent([]byte(version.GetVersion())); err != nil {
		return nil, err
	}
	if err := c.WriteLenContent([]byte(version.VERSION)); err != nil {
		return nil, err
	}
	b, err := c.GetShortContent(32)
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	if crypt.Md5(version.GetVersion()) != string(b) {
		logs.Error("The client does not match the server version. The current core version of the client is", version.GetVersion())
		return nil, err
	}
	if _, err := c.Write([]byte(common.Getverifyval(vkey))); err != nil {
		return nil, err
	}
	if s, err := c.ReadFlag(); err != nil {
		return nil, err
	} else if s == common.VERIFY_EER {
		return nil, errors.New(fmt.Sprintf("Validation key %s incorrect", vkey))
	}
	if _, err := c.Write([]byte(connType)); err != nil {
		return nil, err
	}
	c.SetAlive(tp)

	return c, nil
}

//http proxy connection
func NewHttpProxyConn(url *url.URL, remoteAddr string) (net.Conn, error) {
	req, err := http.NewRequest("CONNECT", "http://"+remoteAddr, nil)
	if err != nil {
		return nil, err
	}
	password, _ := url.User.Password()
	req.Header.Set("Authorization", "Basic "+basicAuth(strings.Trim(url.User.Username(), " "), password))
	// we make a http proxy request
	proxyConn, err := net.Dial("tcp", url.Host)
	if err != nil {
		return nil, err
	}
	if err := req.Write(proxyConn); err != nil {
		return nil, err
	}
	res, err := http.ReadResponse(bufio.NewReader(proxyConn), req)
	if err != nil {
		return nil, err
	}
	_ = res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Proxy error " + res.Status)
	}
	return proxyConn, nil
}

//get a basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func getRemoteAddressFromServer(rAddr string, localConn *net.UDPConn, md5Password, role string, add int) error {
	rAddr, err := getNextAddr(rAddr, add)
	if err != nil {
		logs.Error(err)
		return err
	}
	addr, err := net.ResolveUDPAddr("udp", rAddr)
	if err != nil {
		return err
	}
	if _, err := localConn.WriteTo(common.GetWriteStr(md5Password, role), addr); err != nil {
		return err
	}
	return nil
}

func handleP2PUdp(localAddr, rAddr, md5Password, role string) (remoteAddress string, c net.PacketConn, err error) {
	localConn, err := newUdpConnByAddr(localAddr)
	if err != nil {
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 0)
	if err != nil {
		logs.Error(err)
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 1)
	if err != nil {
		logs.Error(err)
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 2)
	if err != nil {
		logs.Error(err)
		return
	}
	var remoteAddr1, remoteAddr2, remoteAddr3 string
	for {
		buf := make([]byte, 1024)
		if n, addr, er := localConn.ReadFromUDP(buf); er != nil {
			err = er
			return
		} else {
			rAddr2, _ := getNextAddr(rAddr, 1)
			rAddr3, _ := getNextAddr(rAddr, 2)
			switch addr.String() {
			case rAddr:
				remoteAddr1 = string(buf[:n])
			case rAddr2:
				remoteAddr2 = string(buf[:n])
			case rAddr3:
				remoteAddr3 = string(buf[:n])
			}
		}
		if remoteAddr1 != "" && remoteAddr2 != "" && remoteAddr3 != "" {
			break
		}
	}
	if remoteAddress, err = sendP2PTestMsg(localConn, remoteAddr1, remoteAddr2, remoteAddr3); err != nil {
		return
	}
	c, err = newUdpConnByAddr(localAddr)
	return
}

func sendP2PTestMsg(localConn *net.UDPConn, remoteAddr1, remoteAddr2, remoteAddr3 string) (string, error) {
	logs.Trace(remoteAddr3, remoteAddr2, remoteAddr1)
	defer localConn.Close()
	isClose := false
	defer func() { isClose = true }()
	interval, err := getAddrInterval(remoteAddr1, remoteAddr2, remoteAddr3)
	if err != nil {
		return "", err
	}
	go func() {
		addr, err := getNextAddr(remoteAddr3, interval)
		if err != nil {
			return
		}
		remoteUdpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return
		}
		logs.Trace("try send test packet to target %s", addr)
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if isClose {
					return
				}
				if _, err := localConn.WriteTo([]byte(common.WORK_P2P_CONNECT), remoteUdpAddr); err != nil {
					return
				}
			}
		}
	}()
	if interval != 0 {
		ip := common.GetIpByAddr(remoteAddr2)
		go func() {
			ports := getRandomPortArr(common.GetPortByAddr(remoteAddr3), common.GetPortByAddr(remoteAddr3)+interval*50)
			for i := 0; i <= 50; i++ {
				go func(port int) {
					trueAddress := ip + ":" + strconv.Itoa(port)
					logs.Trace("try send test packet to target %s", trueAddress)
					remoteUdpAddr, err := net.ResolveUDPAddr("udp", trueAddress)
					if err != nil {
						return
					}
					ticker := time.NewTicker(time.Second * 2)
					defer ticker.Stop()
					for {
						select {
						case <-ticker.C:
							if isClose {
								return
							}
							if _, err := localConn.WriteTo([]byte(common.WORK_P2P_CONNECT), remoteUdpAddr); err != nil {
								return
							}
						}
					}
				}(ports[i])
				time.Sleep(time.Millisecond * 10)
			}
		}()

	}

	buf := make([]byte, 10)
	for {
		localConn.SetReadDeadline(time.Now().Add(time.Second * 10))
		n, addr, err := localConn.ReadFromUDP(buf)
		localConn.SetReadDeadline(time.Time{})
		if err != nil {
			break
		}
		switch string(buf[:n]) {
		case common.WORK_P2P_SUCCESS:
			for i := 20; i > 0; i-- {
				if _, err = localConn.WriteTo([]byte(common.WORK_P2P_END), addr); err != nil {
					return "", err
				}
			}
			return addr.String(), nil
		case common.WORK_P2P_END:
			logs.Trace("Remotely Address %s Reply Packet Successfully Received", addr.String())
			return addr.String(), nil
		case common.WORK_P2P_CONNECT:
			go func() {
				for i := 20; i > 0; i-- {
					logs.Trace("try send receive success packet to target %s", addr.String())
					if _, err = localConn.WriteTo([]byte(common.WORK_P2P_SUCCESS), addr); err != nil {
						return
					}
					time.Sleep(time.Second)
				}
			}()
		default:
			continue
		}
	}
	return "", errors.New("connect to the target failed, maybe the nat type is not support p2p")
}

func newUdpConnByAddr(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	return udpConn, nil
}

func getNextAddr(addr string, n int) (string, error) {
	arr := strings.Split(addr, ":")
	if len(arr) != 2 {
		return "", errors.New(fmt.Sprintf("the format of %s incorrect", addr))
	}
	if p, err := strconv.Atoi(arr[1]); err != nil {
		return "", err
	} else {
		return arr[0] + ":" + strconv.Itoa(p+n), nil
	}
}

func getAddrInterval(addr1, addr2, addr3 string) (int, error) {
	arr1 := strings.Split(addr1, ":")
	if len(arr1) != 2 {
		return 0, errors.New(fmt.Sprintf("the format of %s incorrect", addr1))
	}
	arr2 := strings.Split(addr2, ":")
	if len(arr2) != 2 {
		return 0, errors.New(fmt.Sprintf("the format of %s incorrect", addr2))
	}
	arr3 := strings.Split(addr3, ":")
	if len(arr3) != 2 {
		return 0, errors.New(fmt.Sprintf("the format of %s incorrect", addr3))
	}
	p1, err := strconv.Atoi(arr1[1])
	if err != nil {
		return 0, err
	}
	p2, err := strconv.Atoi(arr2[1])
	if err != nil {
		return 0, err
	}
	p3, err := strconv.Atoi(arr3[1])
	if err != nil {
		return 0, err
	}
	interVal := int(math.Floor(math.Min(math.Abs(float64(p3-p2)), math.Abs(float64(p2-p1)))))
	if p3-p1 < 0 {
		return -interVal, nil
	}
	return interVal, nil
}

func getRandomPortArr(min, max int) []int {
	if min > max {
		min, max = max, min
	}
	addrAddr := make([]int, max-min+1)
	for i := min; i <= max; i++ {
		addrAddr[max-i] = i
	}
	rand.Seed(time.Now().UnixNano())
	var r, temp int
	for i := max - min; i > 0; i-- {
		r = rand.Int() % i
		temp = addrAddr[i]
		addrAddr[i] = addrAddr[r]
		addrAddr[r] = temp
	}
	return addrAddr
}
