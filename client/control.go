package client

import (
	"bufio"
	"crypto/tls"
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
	"sync"
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

// StartFromFile 从配置文件启动客户端。server_addr 支持配置多个服务端，
// 客户端会与每个服务端建立独立的控制通道与隧道，实现多服务端穿透。
func StartFromFile(path string) {
	cnf, err := config.NewConfig(path)
	if err != nil || cnf.CommonConfig == nil {
		logs.Error("Config file %s loading error %v", path, err)
		return
	}
	servers := cnf.CommonConfig.Servers
	if len(servers) == 0 && cnf.CommonConfig.Server != "" {
		servers = []string{cnf.CommonConfig.Server}
	}
	if len(servers) == 0 {
		logs.Error("Config file %s has no server_addr", path)
		return
	}
	logs.Info("Loading configuration file %s successfully, servers: %s", path, strings.Join(servers, ", "))
	// conn_type=tls 时是否校验服务端证书由配置文件决定（自签部署默认不校验）
	SetTlsVerify(cnf.CommonConfig.TlsVerify)
	// secret / p2p 的本地监听端口与具体服务端无关，只启动一次
	for _, v := range cnf.LocalServer {
		go StartLocalServer(v, cnf.CommonConfig)
	}
	var wg sync.WaitGroup
	for i, addr := range servers {
		wg.Add(1)
		go func(idx int, serverAddr string) {
			defer wg.Done()
			startFromServer(cnf, serverAddr, idx == 0)
		}(i, addr)
	}
	wg.Wait()
}

// startFromServer 与单个服务端注册配置并维持控制通道，断线后按需重连
func startFromServer(cnf *config.Config, serverAddr string, primary bool) {
	first := true
	var fileServers []*http.Server
	defer func() {
		for _, srv := range fileServers {
			_ = srv.Close()
		}
	}()

re:
	if first || cnf.CommonConfig.AutoReconnection {
		if !first {
			logs.Info("[%s] Reconnecting...", serverAddr)
			time.Sleep(time.Second * 5)
		}
	} else {
		return
	}
	first = false
	c, err := NewConn(cnf.CommonConfig.Tp, cnf.CommonConfig.VKey, serverAddr, common.WORK_CONFIG, cnf.CommonConfig.ProxyUrl)
	if err != nil {
		logs.Error("[%s] %s", serverAddr, err.Error())
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
	if primary {
		ioutil.WriteFile(filepath.Join(common.GetTmpPath(), "npc_vkey.txt"), []byte(vkey), 0600)
	}

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
			if srv := startLocalFileServer(cnf.CommonConfig, v, vkey, serverAddr); srv != nil {
				fileServers = append(fileServers, srv)
			}
		}
	}

	c.Close()
	if primary {
		if cnf.CommonConfig.Client.WebUserName == "" || cnf.CommonConfig.Client.WebPassword == "" {
			logs.Notice("web access login username:user password:%s", vkey)
		} else {
			logs.Notice("web access login username:%s password:%s", cnf.CommonConfig.Client.WebUserName, cnf.CommonConfig.Client.WebPassword)
		}
	}
	NewRPClient(serverAddr, vkey, cnf.CommonConfig.Tp, cnf.CommonConfig.ProxyUrl, cnf, cnf.CommonConfig.DisconnectTime).Start()
	for _, srv := range fileServers {
		_ = srv.Close()
	}
	fileServers = nil
	goto re
}

// Create a new connection with the server and verify it
// tlsVerify 控制 tls 连接类型下是否校验服务端证书
var tlsVerify bool

// GetTlsVerify 返回客户端是否校验服务端证书（tls 连接类型）
func GetTlsVerify() bool {
	return tlsVerify
}

// SetTlsVerify 设置是否校验服务端证书。nps 默认使用自签证书，故默认为 false。
func SetTlsVerify(v bool) {
	tlsVerify = v
}

// dialBridge 建立到服务端的原始 TCP 连接，支持 socks5 / http 代理
func dialBridge(proxyUrl, server string) (net.Conn, error) {
	if proxyUrl == "" {
		return net.Dial("tcp", server)
	}
	u, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "socks5":
		n, err := proxy.FromURL(u, nil)
		if err != nil {
			return nil, err
		}
		return n.Dial("tcp", server)
	default:
		return NewHttpProxyConn(u, server)
	}
}

func NewConn(tp string, vkey string, server string, connType string, proxyUrl string) (*conn.Conn, error) {
	var err error
	var connection net.Conn
	var sess *kcp.UDPSession
	switch tp {
	case "kcp":
		sess, err = kcp.DialWithOptions(server, nil, 10, 3)
		if err == nil {
			conn.SetUdpSession(sess)
			connection = sess
		}
	case "tls":
		var raw net.Conn
		if raw, err = dialBridge(proxyUrl, server); err == nil {
			tlsConn := tls.Client(raw, crypt.ClientTlsConfig(server, GetTlsVerify()))
			if herr := tlsConn.Handshake(); herr != nil {
				_ = raw.Close()
				err = herr
			} else {
				connection = tlsConn
			}
		}
	default:
		connection, err = dialBridge(proxyUrl, server)
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

// http proxy connection
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

// get a basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func getRemoteAddressFromServer(rAddr string, localConn *net.UDPConn, md5Password, role string, add int) error {
	rAddr, err := getNextAddr(rAddr, add)
	if err != nil {
		// 这里不再单独打日志：同一个错误会在 handleP2PUdp 的调用方再打一次，
		// 之前一次打洞失败会刷出 4 条一模一样的 `strconv.Atoi: parsing ""`。
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

// p2pCollectTimeout 等待服务端回传三个公网映射地址的上限。
// 旧实现没有超时，服务端 UDP 不通时会永久阻塞在这个循环里（goroutine 与本地端口都不释放）。
const p2pCollectTimeout = 15 * time.Second

func handleP2PUdp(localAddr, rAddr, md5Password, role string) (remoteAddress string, c net.PacketConn, err error) {
	// 先把服务端下发的地址校验并规范化：服务端没开 p2p 时会下发空地址，
	// 旧代码会一路走到 strconv.Atoi 并报出与实际原因无关的解析错误。
	host, port, err := splitP2PAddr(rAddr)
	if err != nil {
		return "", nil, err
	}
	rAddr = net.JoinHostPort(host, strconv.Itoa(port))
	localConn, err := newUdpConnByAddr(localAddr)
	if err != nil {
		return
	}
	// 失败路径必须释放这个临时 socket，否则每轮重试都会漏一个 FD
	defer func() { _ = localConn.Close() }()
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 0)
	if err != nil {
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 1)
	if err != nil {
		return
	}
	err = getRemoteAddressFromServer(rAddr, localConn, md5Password, role, 2)
	if err != nil {
		return
	}
	var remoteAddr1, remoteAddr2, remoteAddr3 string
	// 服务端 UDP 不可达时旧实现会永久阻塞在这个循环里；这里加超时并给出可读原因
	_ = localConn.SetReadDeadline(time.Now().Add(p2pCollectTimeout))
	for {
		buf := make([]byte, 1024)
		if n, addr, er := localConn.ReadFromUDP(buf); er != nil {
			if ne, ok := er.(net.Error); ok && ne.Timeout() {
				err = fmt.Errorf("p2p: no reply from %s within %s, check that udp port %d-%d is reachable", rAddr, p2pCollectTimeout, port, port+2)
				return
			}
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

// errP2PAddrInvalid 表示服务端下发的 p2p 地址不可用。
// 最常见的原因是服务端 nps.conf 里 p2p_ip / p2p_port 没配置：旧代码把两者直接拼接，
// 得到 ":" 这样的地址，再被 strings.Split 切成两个空串，最后抛出
// `strconv.Atoi: parsing "": invalid syntax`——从错误信息完全看不出跟 p2p 有关。
// 这类错误重试不会自愈，调用方应降频提示而不是持续刷日志。
var errP2PAddrInvalid = errors.New("invalid p2p address from server")

// splitP2PAddr 解析服务端下发的 "host:port"。
// 用 net.SplitHostPort 而不是 strings.Split(":")：后者会把 `[::1]:6000` 切成多段，
// 把合法的 IPv6 地址误判成格式错误。
func splitP2PAddr(addr string) (string, int, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0, fmt.Errorf("%w: server sent an empty address, enable p2p by setting p2p_ip and p2p_port in the server nps.conf", errP2PAddrInvalid)
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %q, expected host:port", errP2PAddrInvalid, addr)
	}
	if host == "" {
		return "", 0, fmt.Errorf("%w: %q has no host, set p2p_ip in the server nps.conf", errP2PAddrInvalid, addr)
	}
	if strings.TrimSpace(portStr) == "" {
		return "", 0, fmt.Errorf("%w: %q has no port, set p2p_port in the server nps.conf", errP2PAddrInvalid, addr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("%w: %q has an invalid port", errP2PAddrInvalid, addr)
	}
	return host, port, nil
}

func getNextAddr(addr string, n int) (string, error) {
	host, port, err := splitP2PAddr(addr)
	if err != nil {
		return "", err
	}
	next := port + n
	if next <= 0 || next > 65535 {
		return "", fmt.Errorf("%w: port %d out of range after offset %d", errP2PAddrInvalid, next, n)
	}
	return net.JoinHostPort(host, strconv.Itoa(next)), nil
}

func getAddrInterval(addr1, addr2, addr3 string) (int, error) {
	_, p1, err := splitP2PAddr(addr1)
	if err != nil {
		return 0, err
	}
	_, p2, err := splitP2PAddr(addr2)
	if err != nil {
		return 0, err
	}
	_, p3, err := splitP2PAddr(addr3)
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
