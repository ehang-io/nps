package bridge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/mux"
	"github.com/cnlh/nps/lib/version"
	"github.com/cnlh/nps/server/connection"
	"github.com/cnlh/nps/server/tool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	tunnel    *mux.Mux
	signal    *conn.Conn
	file      *mux.Mux
	retryTime int // it will be add 1 when ping not ok until to 3 will close the client
	sync.RWMutex
}

func NewClient(t, f *mux.Mux, s *conn.Conn) *Client {
	return &Client{
		signal: s,
		tunnel: t,
		file:   f,
	}
}

type Bridge struct {
	TunnelPort   int //通信隧道端口
	Client       map[int]*Client
	tunnelType   string //bridge type kcp or tcp
	OpenTask     chan *file.Tunnel
	CloseTask    chan *file.Tunnel
	CloseClient  chan int
	SecretChan   chan *conn.Secret
	clientLock   sync.RWMutex
	Register     map[string]time.Time
	registerLock sync.RWMutex
	ipVerify     bool
	runList      map[int]interface{}
}

func NewTunnel(tunnelPort int, tunnelType string, ipVerify bool, runList map[int]interface{}) *Bridge {
	t := new(Bridge)
	t.TunnelPort = tunnelPort
	t.Client = make(map[int]*Client)
	t.tunnelType = tunnelType
	t.OpenTask = make(chan *file.Tunnel)
	t.CloseTask = make(chan *file.Tunnel)
	t.CloseClient = make(chan int)
	t.Register = make(map[string]time.Time)
	t.ipVerify = ipVerify
	t.runList = runList
	t.SecretChan = make(chan *conn.Secret)
	return t
}

func (s *Bridge) StartTunnel() error {
	go s.ping()
	if s.tunnelType == "kcp" {
		listener, err := kcp.ListenWithOptions(beego.AppConfig.String("bridge_ip")+":"+beego.AppConfig.String("bridge_port"), nil, 150, 3)
		if err != nil {
			logs.Error(err)
			os.Exit(0)
			return err
		}
		logs.Info("server start, the bridge type is %s, the bridge port is %d", s.tunnelType, s.TunnelPort)
		go func() {
			for {
				c, err := listener.AcceptKCP()
				conn.SetUdpSession(c)
				if err != nil {
					logs.Warn(err)
					continue
				}
				go s.cliProcess(conn.NewConn(c))
			}
		}()
	} else {
		listener, err := connection.GetBridgeListener(s.tunnelType)
		if err != nil {
			logs.Error(err)
			os.Exit(0)
			return err
		}
		go func() {
			for {
				c, err := listener.Accept()
				if err != nil {
					logs.Warn(err)
					continue
				}
				go s.cliProcess(conn.NewConn(c))
			}
		}()
	}
	return nil
}

//get health information form client
func (s *Bridge) GetHealthFromClient(id int, c *conn.Conn) {
	for {
		if info, status, err := c.GetHealthInfo(); err != nil {
			logs.Error(err)
			break
		} else if !status { //the status is true , return target to the targetArr
			for _, v := range file.GetCsvDb().Tasks {
				if v.Client.Id == id && v.Mode == "tcp" && strings.Contains(v.Target, info) {
					v.Lock()
					if v.TargetArr == nil || (len(v.TargetArr) == 0 && len(v.HealthRemoveArr) == 0) {
						v.TargetArr = common.TrimArr(strings.Split(v.Target, "\n"))
					}
					v.TargetArr = common.RemoveArrVal(v.TargetArr, info)
					if v.HealthRemoveArr == nil {
						v.HealthRemoveArr = make([]string, 0)
					}
					v.HealthRemoveArr = append(v.HealthRemoveArr, info)
					v.Unlock()
				}
			}
			for _, v := range file.GetCsvDb().Hosts {
				if v.Client.Id == id && strings.Contains(v.Target, info) {
					v.Lock()
					if v.TargetArr == nil || (len(v.TargetArr) == 0 && len(v.HealthRemoveArr) == 0) {
						v.TargetArr = common.TrimArr(strings.Split(v.Target, "\n"))
					}
					v.TargetArr = common.RemoveArrVal(v.TargetArr, info)
					if v.HealthRemoveArr == nil {
						v.HealthRemoveArr = make([]string, 0)
					}
					v.HealthRemoveArr = append(v.HealthRemoveArr, info)
					v.Unlock()
				}
			}
		} else { //the status is false,remove target from the targetArr
			for _, v := range file.GetCsvDb().Tasks {
				if v.Client.Id == id && v.Mode == "tcp" && common.IsArrContains(v.HealthRemoveArr, info) && !common.IsArrContains(v.TargetArr, info) {
					v.Lock()
					v.TargetArr = append(v.TargetArr, info)
					v.HealthRemoveArr = common.RemoveArrVal(v.HealthRemoveArr, info)
					v.Unlock()
				}
			}
			for _, v := range file.GetCsvDb().Hosts {
				if v.Client.Id == id && common.IsArrContains(v.HealthRemoveArr, info) && !common.IsArrContains(v.TargetArr, info) {
					v.Lock()
					v.TargetArr = append(v.TargetArr, info)
					v.HealthRemoveArr = common.RemoveArrVal(v.HealthRemoveArr, info)
					v.Unlock()
				}
			}
		}
	}
}

//验证失败，返回错误验证flag，并且关闭连接
func (s *Bridge) verifyError(c *conn.Conn) {
	c.Write([]byte(common.VERIFY_EER))
	c.Conn.Close()
}

func (s *Bridge) verifySuccess(c *conn.Conn) {
	c.Write([]byte(common.VERIFY_SUCCESS))
}

func (s *Bridge) cliProcess(c *conn.Conn) {
	//read test flag
	if _, err := c.GetShortContent(3); err != nil {
		logs.Info("The client %s connect error", c.Conn.RemoteAddr())
		return
	}
	//version check
	if b, err := c.GetShortContent(32); err != nil || string(b) != crypt.Md5(version.GetVersion()) {
		logs.Info("The client %s version does not match", c.Conn.RemoteAddr())
		c.Close()
		return
	}
	//write server version to client
	c.Write([]byte(crypt.Md5(version.GetVersion())))
	c.SetReadDeadline(5, s.tunnelType)
	var buf []byte
	var err error
	//get vKey from client
	if buf, err = c.GetShortContent(32); err != nil {
		c.Close()
		return
	}
	//verify
	id, err := file.GetCsvDb().GetIdByVerifyKey(string(buf), c.Conn.RemoteAddr().String())
	if err != nil {
		logs.Info("Current client connection validation error, close this client:", c.Conn.RemoteAddr())
		s.verifyError(c)
		return
	} else {
		s.verifySuccess(c)
	}
	if flag, err := c.ReadFlag(); err == nil {
		s.typeDeal(flag, c, id)
	} else {
		logs.Warn(err, flag)
	}
	return
}

func (s *Bridge) DelClient(id int, isOther bool) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	if v, ok := s.Client[id]; ok {
		if c, err := file.GetCsvDb().GetClient(id); err == nil && c.NoStore {
			s.CloseClient <- c.Id
		}
		if v.signal != nil {
			v.signal.Close()
		}
		delete(s.Client, id)
	}
}

//use different
func (s *Bridge) typeDeal(typeVal string, c *conn.Conn, id int) {
	switch typeVal {
	case common.WORK_MAIN:
		//the vKey connect by another ,close the client of before
		s.clientLock.Lock()
		if v, ok := s.Client[id]; ok {
			s.clientLock.Unlock()
			if v.signal != nil {
				v.signal.WriteClose()
			}
			v.Lock()
			v.signal = c
			v.Unlock()
		} else {
			s.Client[id] = NewClient(nil, nil, c)
			s.clientLock.Unlock()
		}
		go s.GetHealthFromClient(id, c)
		logs.Info("clientId %d connection succeeded, address:%s ", id, c.Conn.RemoteAddr())
	case common.WORK_CHAN:
		s.clientLock.Lock()
		if v, ok := s.Client[id]; ok {
			s.clientLock.Unlock()
			v.Lock()
			v.tunnel = mux.NewMux(c.Conn, s.tunnelType)
			v.Unlock()
		} else {
			s.Client[id] = NewClient(mux.NewMux(c.Conn, s.tunnelType), nil, nil)
			s.clientLock.Unlock()
		}
	case common.WORK_CONFIG:
		var isPub bool
		client, err := file.GetCsvDb().GetClient(id);
		if err == nil {
			if client.VerifyKey == beego.AppConfig.String("public_vkey") {
				isPub = true
			} else {
				isPub = false
			}
		}
		binary.Write(c, binary.LittleEndian, isPub)
		go s.getConfig(c, isPub, client)
	case common.WORK_REGISTER:
		go s.register(c)
	case common.WORK_SECRET:
		if b, err := c.GetShortContent(32); err == nil {
			s.SecretChan <- conn.NewSecret(string(b), c)
		}
	case common.WORK_FILE:
		s.clientLock.Lock()
		if v, ok := s.Client[id]; ok {
			s.clientLock.Unlock()
			v.Lock()
			v.file = mux.NewMux(c.Conn, s.tunnelType)
			v.Unlock()
		} else {
			s.Client[id] = NewClient(nil, mux.NewMux(c.Conn, s.tunnelType), nil)
			s.clientLock.Unlock()
		}
	case common.WORK_P2P:
		//read md5 secret
		if b, err := c.GetShortContent(32); err != nil {
			return
		} else if t := file.GetCsvDb().GetTaskByMd5Password(string(b)); t == nil {
			return
		} else {
			s.clientLock.Lock()
			if v, ok := s.Client[t.Client.Id]; !ok {
				s.clientLock.Unlock()
				return
			} else {
				s.clientLock.Unlock()
				//向密钥对应的客户端发送与服务端udp建立连接信息，地址，密钥
				v.signal.Write([]byte(common.NEW_UDP_CONN))
				svrAddr := beego.AppConfig.String("p2p_ip") + ":" + beego.AppConfig.String("p2p_port")
				if err != nil {
					logs.Warn("get local udp addr error")
					return
				}
				v.signal.WriteLenContent([]byte(svrAddr))
				v.signal.WriteLenContent(b)
				//向该请求者发送建立连接请求,服务器地址
				c.WriteLenContent([]byte(svrAddr))
			}
		}
	}
	c.SetAlive(s.tunnelType)
	return
}

//register ip
func (s *Bridge) register(c *conn.Conn) {
	var hour int32
	if err := binary.Read(c, binary.LittleEndian, &hour); err == nil {
		s.registerLock.Lock()
		s.Register[common.GetIpByAddr(c.Conn.RemoteAddr().String())] = time.Now().Add(time.Hour * time.Duration(hour))
		s.registerLock.Unlock()
	}
}

func (s *Bridge) SendLinkInfo(clientId int, link *conn.Link, linkAddr string, t *file.Tunnel) (target net.Conn, err error) {
	s.clientLock.Lock()
	if v, ok := s.Client[clientId]; ok {
		s.clientLock.Unlock()

		//If ip is restricted to do ip verification
		if s.ipVerify {
			s.registerLock.Lock()
			ip := common.GetIpByAddr(linkAddr)
			if v, ok := s.Register[ip]; !ok {
				s.registerLock.Unlock()
				return nil, errors.New(fmt.Sprintf("The ip %s is not in the validation list", ip))
			} else {
				s.registerLock.Unlock()
				if !v.After(time.Now()) {
					return nil, errors.New(fmt.Sprintf("The validity of the ip %s has expired", ip))
				}
			}
		}
		var tunnel *mux.Mux
		if t != nil && t.Mode == "file" {
			tunnel = v.file
		} else {
			tunnel = v.tunnel
		}
		if tunnel == nil {
			err = errors.New("the client connect error")
			return
		}

		if target, err = tunnel.NewConn(); err != nil {
			return
		}

		if t != nil && t.Mode == "file" {
			return
		}

		if _, err = conn.NewConn(target).SendLinkInfo(link); err != nil {
			logs.Info("new connect error ,the target %s refuse to connect", link.Host)
			return
		}
	} else {
		s.clientLock.Unlock()
		err = errors.New(fmt.Sprintf("the client %d is not connect", clientId))
	}
	return
}

func (s *Bridge) ping() {
	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-ticker.C:
			s.clientLock.Lock()
			arr := make([]int, 0)
			for k, v := range s.Client {
				if v.tunnel == nil || v.signal == nil {
					v.retryTime += 1
					if v.retryTime >= 3 {
						arr = append(arr, k)
					}
					continue
				}
				if v.tunnel.IsClose {
					arr = append(arr, k)
				}
			}
			s.clientLock.Unlock()
			for _, v := range arr {
				logs.Info("the client %d closed", v)
				s.DelClient(v, false)
			}
		}
	}
}

//get config and add task from client config
func (s *Bridge) getConfig(c *conn.Conn, isPub bool, client *file.Client) {
	var fail bool
loop:
	for {
		flag, err := c.ReadFlag()
		if err != nil {
			break
		}
		switch flag {
		case common.WORK_STATUS:
			if b, err := c.GetShortContent(32); err != nil {
				break loop
			} else {
				var str string
				id, err := file.GetCsvDb().GetClientIdByVkey(string(b))
				if err != nil {
					break loop
				}
				file.GetCsvDb().Lock()
				for _, v := range file.GetCsvDb().Hosts {
					if v.Client.Id == id {
						str += v.Remark + common.CONN_DATA_SEQ
					}
				}
				for _, v := range file.GetCsvDb().Tasks {
					if _, ok := s.runList[v.Id]; ok && v.Client.Id == id {
						str += v.Remark + common.CONN_DATA_SEQ
					}
				}
				file.GetCsvDb().Unlock()
				binary.Write(c, binary.LittleEndian, int32(len([]byte(str))))
				binary.Write(c, binary.LittleEndian, []byte(str))
			}
		case common.NEW_CONF:
			var err error
			if client, err = c.GetConfigInfo(); err != nil {
				fail = true
				c.WriteAddFail()
				break loop
			} else {
				if err = file.GetCsvDb().NewClient(client); err != nil {
					fail = true
					c.WriteAddFail()
					break loop
				}
				c.WriteAddOk()
				c.Write([]byte(client.VerifyKey))
				s.clientLock.Lock()
				s.Client[client.Id] = NewClient(nil, nil, nil)
				s.clientLock.Unlock()
			}
		case common.NEW_HOST:
			h, err := c.GetHostInfo()
			if err != nil {
				fail = true
				c.WriteAddFail()
				break loop
			}
			h.Client = client
			if h.Location == "" {
				h.Location = "/"
			}
			if !client.HasHost(h) {
				if file.GetCsvDb().IsHostExist(h) {
					fail = true
					c.WriteAddFail()
					break loop
				} else {
					file.GetCsvDb().NewHost(h)
					c.WriteAddOk()
				}
			} else {
				c.WriteAddOk()
			}
		case common.NEW_TASK:
			if t, err := c.GetTaskInfo(); err != nil {
				fail = true
				c.WriteAddFail()
				break loop
			} else {
				ports := common.GetPorts(t.Ports)
				targets := common.GetPorts(t.Target)
				if len(ports) > 1 && (t.Mode == "tcp" || t.Mode == "udp") && (len(ports) != len(targets)) {
					fail = true
					c.WriteAddFail()
					break loop
				} else if t.Mode == "secret" {
					ports = append(ports, 0)
				}
				if len(ports) == 0 {
					fail = true
					c.WriteAddFail()
					break loop
				}
				for i := 0; i < len(ports); i++ {
					tl := new(file.Tunnel)
					tl.Mode = t.Mode
					tl.Port = ports[i]
					if len(ports) == 1 {
						tl.Target = t.Target
						tl.Remark = t.Remark
					} else {
						tl.Remark = t.Remark + "_" + strconv.Itoa(tl.Port)
						if t.TargetAddr != "" {
							tl.Target = t.TargetAddr + ":" + strconv.Itoa(targets[i])
						} else {
							tl.Target = strconv.Itoa(targets[i])
						}
					}
					tl.Id = file.GetCsvDb().GetTaskId()
					tl.Status = true
					tl.Flow = new(file.Flow)
					tl.NoStore = true
					tl.Client = client
					tl.Password = t.Password
					tl.LocalPath = t.LocalPath
					tl.StripPre = t.StripPre
					if !client.HasTunnel(tl) {
						if err := file.GetCsvDb().NewTask(tl); err != nil {
							logs.Notice("Add task error ", err.Error())
							fail = true
							c.WriteAddFail()
							break loop
						}
						if b := tool.TestServerPort(tl.Port, tl.Mode); !b && t.Mode != "secret" && t.Mode != "p2p" {
							fail = true
							c.WriteAddFail()
							break loop
						} else {
							s.OpenTask <- tl
						}
					}
					c.WriteAddOk()
				}
			}
		}
	}
	if fail && client != nil {
		s.DelClient(client.Id, false)
	}
	c.Close()
}
