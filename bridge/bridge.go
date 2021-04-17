package bridge

import (
	"ehang.io/nps-mux"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"ehang.io/nps/server/connection"
	"ehang.io/nps/server/tool"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type Client struct {
	tunnel    *nps_mux.Mux
	signal    *conn.Conn
	file      *nps_mux.Mux
	Version   string
	retryTime int // it will be add 1 when ping not ok until to 3 will close the client
}

func NewClient(t, f *nps_mux.Mux, s *conn.Conn, vs string) *Client {
	return &Client{
		signal:  s,
		tunnel:  t,
		file:    f,
		Version: vs,
	}
}

type Bridge struct {
	TunnelPort     int //通信隧道端口
	Client         sync.Map
	Register       sync.Map
	tunnelType     string //bridge type kcp or tcp
	OpenTask       chan *file.Tunnel
	CloseTask      chan *file.Tunnel
	CloseClient    chan int
	SecretChan     chan *conn.Secret
	ipVerify       bool
	runList        sync.Map //map[int]interface{}
	disconnectTime int
}

func NewTunnel(tunnelPort int, tunnelType string, ipVerify bool, runList sync.Map, disconnectTime int) *Bridge {
	return &Bridge{
		TunnelPort:     tunnelPort,
		tunnelType:     tunnelType,
		OpenTask:       make(chan *file.Tunnel),
		CloseTask:      make(chan *file.Tunnel),
		CloseClient:    make(chan int),
		SecretChan:     make(chan *conn.Secret),
		ipVerify:       ipVerify,
		runList:        runList,
		disconnectTime: disconnectTime,
	}
}

func (s *Bridge) StartTunnel() error {
	go s.ping()
	if s.tunnelType == "kcp" {
		logs.Info("server start, the bridge type is %s, the bridge port is %d", s.tunnelType, s.TunnelPort)
		return conn.NewKcpListenerAndProcess(beego.AppConfig.String("bridge_ip")+":"+beego.AppConfig.String("bridge_port"), func(c net.Conn) {
			s.cliProcess(conn.NewConn(c))
		})
	} else {
		listener, err := connection.GetBridgeListener(s.tunnelType)
		if err != nil {
			logs.Error(err)
			os.Exit(0)
			return err
		}
		conn.Accept(listener, func(c net.Conn) {
			s.cliProcess(conn.NewConn(c))
		})
	}
	return nil
}

//get health information form client
func (s *Bridge) GetHealthFromClient(id int, c *conn.Conn) {
	for {
		if info, status, err := c.GetHealthInfo(); err != nil {
			break
		} else if !status { //the status is true , return target to the targetArr
			file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
				v := value.(*file.Tunnel)
				if v.Client.Id == id && v.Mode == "tcp" && strings.Contains(v.Target.TargetStr, info) {
					v.Lock()
					if v.Target.TargetArr == nil || (len(v.Target.TargetArr) == 0 && len(v.HealthRemoveArr) == 0) {
						v.Target.TargetArr = common.TrimArr(strings.Split(v.Target.TargetStr, "\n"))
					}
					v.Target.TargetArr = common.RemoveArrVal(v.Target.TargetArr, info)
					if v.HealthRemoveArr == nil {
						v.HealthRemoveArr = make([]string, 0)
					}
					v.HealthRemoveArr = append(v.HealthRemoveArr, info)
					v.Unlock()
				}
				return true
			})
			file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
				v := value.(*file.Host)
				if v.Client.Id == id && strings.Contains(v.Target.TargetStr, info) {
					v.Lock()
					if v.Target.TargetArr == nil || (len(v.Target.TargetArr) == 0 && len(v.HealthRemoveArr) == 0) {
						v.Target.TargetArr = common.TrimArr(strings.Split(v.Target.TargetStr, "\n"))
					}
					v.Target.TargetArr = common.RemoveArrVal(v.Target.TargetArr, info)
					if v.HealthRemoveArr == nil {
						v.HealthRemoveArr = make([]string, 0)
					}
					v.HealthRemoveArr = append(v.HealthRemoveArr, info)
					v.Unlock()
				}
				return true
			})
		} else { //the status is false,remove target from the targetArr
			file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
				v := value.(*file.Tunnel)
				if v.Client.Id == id && v.Mode == "tcp" && common.IsArrContains(v.HealthRemoveArr, info) && !common.IsArrContains(v.Target.TargetArr, info) {
					v.Lock()
					v.Target.TargetArr = append(v.Target.TargetArr, info)
					v.HealthRemoveArr = common.RemoveArrVal(v.HealthRemoveArr, info)
					v.Unlock()
				}
				return true
			})

			file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
				v := value.(*file.Host)
				if v.Client.Id == id && common.IsArrContains(v.HealthRemoveArr, info) && !common.IsArrContains(v.Target.TargetArr, info) {
					v.Lock()
					v.Target.TargetArr = append(v.Target.TargetArr, info)
					v.HealthRemoveArr = common.RemoveArrVal(v.HealthRemoveArr, info)
					v.Unlock()
				}
				return true
			})
		}
	}
	s.DelClient(id)
}

//验证失败，返回错误验证flag，并且关闭连接
func (s *Bridge) verifyError(c *conn.Conn) {
	c.Write([]byte(common.VERIFY_EER))
}

func (s *Bridge) verifySuccess(c *conn.Conn) {
	c.Write([]byte(common.VERIFY_SUCCESS))
}

func (s *Bridge) cliProcess(c *conn.Conn) {
	//read test flag
	if _, err := c.GetShortContent(3); err != nil {
		logs.Info("The client %s connect error", c.Conn.RemoteAddr(), err.Error())
		return
	}
	//version check
	if b, err := c.GetShortLenContent(); err != nil || string(b) != version.GetVersion() {
		logs.Info("The client %s version does not match", c.Conn.RemoteAddr())
		c.Close()
		return
	}
	//version get
	var vs []byte
	var err error
	if vs, err = c.GetShortLenContent(); err != nil {
		logs.Info("get client %s version error", err.Error())
		c.Close()
		return
	}
	//write server version to client
	c.Write([]byte(crypt.Md5(version.GetVersion())))
	c.SetReadDeadlineBySecond(5)
	var buf []byte
	//get vKey from client
	if buf, err = c.GetShortContent(32); err != nil {
		c.Close()
		return
	}
	//verify
	id, err := file.GetDb().GetIdByVerifyKey(string(buf), c.Conn.RemoteAddr().String())
	if err != nil {
		logs.Info("Current client connection validation error, close this client:", c.Conn.RemoteAddr())
		s.verifyError(c)
		return
	} else {
		s.verifySuccess(c)
	}
	if flag, err := c.ReadFlag(); err == nil {
		s.typeDeal(flag, c, id, string(vs))
	} else {
		logs.Warn(err, flag)
	}
	return
}

func (s *Bridge) DelClient(id int) {
	if v, ok := s.Client.Load(id); ok {
		if v.(*Client).signal != nil {
			v.(*Client).signal.Close()
		}
		s.Client.Delete(id)
		if file.GetDb().IsPubClient(id) {
			return
		}
		if c, err := file.GetDb().GetClient(id); err == nil {
			s.CloseClient <- c.Id
		}
	}
}

//use different
func (s *Bridge) typeDeal(typeVal string, c *conn.Conn, id int, vs string) {
	isPub := file.GetDb().IsPubClient(id)
	switch typeVal {
	case common.WORK_MAIN:
		if isPub {
			c.Close()
			return
		}
		tcpConn, ok := c.Conn.(*net.TCPConn)
		if ok {
			// add tcp keep alive option for signal connection
			_ = tcpConn.SetKeepAlive(true)
			_ = tcpConn.SetKeepAlivePeriod(5 * time.Second)
		}
		//the vKey connect by another ,close the client of before
		if v, ok := s.Client.LoadOrStore(id, NewClient(nil, nil, c, vs)); ok {
			if v.(*Client).signal != nil {
				v.(*Client).signal.WriteClose()
			}
			v.(*Client).signal = c
			v.(*Client).Version = vs
		}
		go s.GetHealthFromClient(id, c)
		logs.Info("clientId %d connection succeeded, address:%s ", id, c.Conn.RemoteAddr())
	case common.WORK_CHAN:
		muxConn := nps_mux.NewMux(c.Conn, s.tunnelType, s.disconnectTime)
		if v, ok := s.Client.LoadOrStore(id, NewClient(muxConn, nil, nil, vs)); ok {
			v.(*Client).tunnel = muxConn
		}
	case common.WORK_CONFIG:
		client, err := file.GetDb().GetClient(id)
		if err != nil || (!isPub && !client.ConfigConnAllow) {
			c.Close()
			return
		}
		binary.Write(c, binary.LittleEndian, isPub)
		go s.getConfig(c, isPub, client)
	case common.WORK_REGISTER:
		go s.register(c)
	case common.WORK_SECRET:
		if b, err := c.GetShortContent(32); err == nil {
			s.SecretChan <- conn.NewSecret(string(b), c)
		} else {
			logs.Error("secret error, failed to match the key successfully")
		}
	case common.WORK_FILE:
		muxConn := nps_mux.NewMux(c.Conn, s.tunnelType, s.disconnectTime)
		if v, ok := s.Client.LoadOrStore(id, NewClient(nil, muxConn, nil, vs)); ok {
			v.(*Client).file = muxConn
		}
	case common.WORK_P2P:
		//read md5 secret
		if b, err := c.GetShortContent(32); err != nil {
			logs.Error("p2p error,", err.Error())
		} else if t := file.GetDb().GetTaskByMd5Password(string(b)); t == nil {
			logs.Error("p2p error, failed to match the key successfully")
		} else {
			if v, ok := s.Client.Load(t.Client.Id); !ok {
				return
			} else {
				//向密钥对应的客户端发送与服务端udp建立连接信息，地址，密钥
				v.(*Client).signal.Write([]byte(common.NEW_UDP_CONN))
				svrAddr := beego.AppConfig.String("p2p_ip") + ":" + beego.AppConfig.String("p2p_port")
				if err != nil {
					logs.Warn("get local udp addr error")
					return
				}
				v.(*Client).signal.WriteLenContent([]byte(svrAddr))
				v.(*Client).signal.WriteLenContent(b)
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
		s.Register.Store(common.GetIpByAddr(c.Conn.RemoteAddr().String()), time.Now().Add(time.Hour*time.Duration(hour)))
	}
}

func (s *Bridge) SendLinkInfo(clientId int, link *conn.Link, t *file.Tunnel) (target net.Conn, err error) {
	//if the proxy type is local
	if link.LocalProxy {
		target, err = net.Dial("tcp", link.Host)
		return
	}
	if v, ok := s.Client.Load(clientId); ok {
		//If ip is restricted to do ip verification
		if s.ipVerify {
			ip := common.GetIpByAddr(link.RemoteAddr)
			if v, ok := s.Register.Load(ip); !ok {
				return nil, errors.New(fmt.Sprintf("The ip %s is not in the validation list", ip))
			} else {
				if !v.(time.Time).After(time.Now()) {
					return nil, errors.New(fmt.Sprintf("The validity of the ip %s has expired", ip))
				}
			}
		}
		var tunnel *nps_mux.Mux
		if t != nil && t.Mode == "file" {
			tunnel = v.(*Client).file
		} else {
			tunnel = v.(*Client).tunnel
		}
		if tunnel == nil {
			err = errors.New("the client connect error")
			return
		}
		if target, err = tunnel.NewConn(); err != nil {
			return
		}
		if t != nil && t.Mode == "file" {
			//TODO if t.mode is file ,not use crypt or compress
			link.Crypt = false
			link.Compress = false
			return
		}
		if _, err = conn.NewConn(target).SendInfo(link, ""); err != nil {
			logs.Info("new connect error ,the target %s refuse to connect", link.Host)
			return
		}
	} else {
		err = errors.New(fmt.Sprintf("the client %d is not connect", clientId))
	}
	return
}

func (s *Bridge) ping() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			arr := make([]int, 0)
			s.Client.Range(func(key, value interface{}) bool {
				v := value.(*Client)
				if v.tunnel == nil || v.signal == nil {
					v.retryTime += 1
					if v.retryTime >= 3 {
						arr = append(arr, key.(int))
					}
					return true
				}
				if v.tunnel.IsClose {
					arr = append(arr, key.(int))
				}
				return true
			})
			for _, v := range arr {
				logs.Info("the client %d closed", v)
				s.DelClient(v)
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
				id, err := file.GetDb().GetClientIdByVkey(string(b))
				if err != nil {
					break loop
				}
				file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
					v := value.(*file.Host)
					if v.Client.Id == id {
						str += v.Remark + common.CONN_DATA_SEQ
					}
					return true
				})
				file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
					v := value.(*file.Tunnel)
					//if _, ok := s.runList[v.Id]; ok && v.Client.Id == id {
					if _, ok := s.runList.Load(v.Id); ok && v.Client.Id == id {
						str += v.Remark + common.CONN_DATA_SEQ
					}
					return true
				})
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
				if err = file.GetDb().NewClient(client); err != nil {
					fail = true
					c.WriteAddFail()
					break loop
				}
				c.WriteAddOk()
				c.Write([]byte(client.VerifyKey))
				s.Client.Store(client.Id, NewClient(nil, nil, nil, ""))
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
				if file.GetDb().IsHostExist(h) {
					fail = true
					c.WriteAddFail()
					break loop
				} else {
					file.GetDb().NewHost(h)
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
				targets := common.GetPorts(t.Target.TargetStr)
				if len(ports) > 1 && (t.Mode == "tcp" || t.Mode == "udp") && (len(ports) != len(targets)) {
					fail = true
					c.WriteAddFail()
					break loop
				} else if t.Mode == "secret" || t.Mode == "p2p" {
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
					tl.ServerIp = t.ServerIp
					if len(ports) == 1 {
						tl.Target = t.Target
						tl.Remark = t.Remark
					} else {
						tl.Remark = t.Remark + "_" + strconv.Itoa(tl.Port)
						tl.Target = new(file.Target)
						if t.TargetAddr != "" {
							tl.Target.TargetStr = t.TargetAddr + ":" + strconv.Itoa(targets[i])
						} else {
							tl.Target.TargetStr = strconv.Itoa(targets[i])
						}
					}
					tl.Id = int(file.GetDb().JsonDb.GetTaskId())
					tl.Status = true
					tl.Flow = new(file.Flow)
					tl.NoStore = true
					tl.Client = client
					tl.Password = t.Password
					tl.LocalPath = t.LocalPath
					tl.StripPre = t.StripPre
					tl.MultiAccount = t.MultiAccount
					if !client.HasTunnel(tl) {
						if err := file.GetDb().NewTask(tl); err != nil {
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
		s.DelClient(client.Id)
	}
	c.Close()
}
