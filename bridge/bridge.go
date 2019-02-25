package bridge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/crypt"
	"github.com/cnlh/nps/lib/file"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/lib/version"
	"github.com/cnlh/nps/server/tool"
	"github.com/cnlh/nps/vender/github.com/astaxie/beego/logs"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"net"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	tunnel        *conn.Conn
	signal        *conn.Conn
	msg           *conn.Conn
	linkMap       map[int]*conn.Link
	linkStatusMap map[int]bool
	stop          chan bool
	sync.RWMutex
}

func NewClient(t *conn.Conn, s *conn.Conn, m *conn.Conn) *Client {
	return &Client{
		linkMap:       make(map[int]*conn.Link),
		stop:          make(chan bool),
		linkStatusMap: make(map[int]bool),
		signal:        s,
		tunnel:        t,
		msg:           m,
	}
}

type Bridge struct {
	TunnelPort   int              //通信隧道端口
	tcpListener  *net.TCPListener //server端监听
	kcpListener  *kcp.Listener    //server端监听
	Client       map[int]*Client
	tunnelType   string //bridge type kcp or tcp
	OpenTask     chan *file.Tunnel
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
	t.CloseClient = make(chan int)
	t.Register = make(map[string]time.Time)
	t.ipVerify = ipVerify
	t.runList = runList
	t.SecretChan = make(chan *conn.Secret)
	return t
}

func (s *Bridge) StartTunnel() error {
	go s.linkCleanSession()
	var err error
	if s.tunnelType == "kcp" {
		s.kcpListener, err = kcp.ListenWithOptions(":"+strconv.Itoa(s.TunnelPort), nil, 150, 3)
		if err != nil {
			return err
		}
		go func() {
			for {
				c, err := s.kcpListener.AcceptKCP()
				conn.SetUdpSession(c)
				if err != nil {
					logs.Warn(err)
					continue
				}
				go s.cliProcess(conn.NewConn(c))
			}
		}()
	} else {
		s.tcpListener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.TunnelPort, ""})
		if err != nil {
			return err
		}
		go func() {
			for {
				c, err := s.tcpListener.Accept()
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

//验证失败，返回错误验证flag，并且关闭连接
func (s *Bridge) verifyError(c *conn.Conn) {
	c.Write([]byte(common.VERIFY_EER))
	c.Conn.Close()
}

func (s *Bridge) verifySuccess(c *conn.Conn) {
	c.Write([]byte(common.VERIFY_SUCCESS))
}

func (s *Bridge) cliProcess(c *conn.Conn) {
	if b, err := c.ReadLen(32); err != nil || string(b) != crypt.Md5(version.GetVersion()) {
		logs.Info("The client %s version does not match", c.Conn.RemoteAddr())
		c.Close()
		return
	}
	c.Write([]byte(crypt.Md5(version.GetVersion())))
	c.SetReadDeadline(5, s.tunnelType)
	var buf []byte
	var err error
	if buf, err = c.ReadLen(32); err != nil {
		c.Close()
		return
	}
	//验证
	id, err := file.GetCsvDb().GetIdByVerifyKey(string(buf), c.Conn.RemoteAddr().String())
	if err != nil {
		logs.Info("Current client connection validation error, close this client:", c.Conn.RemoteAddr())
		s.verifyError(c)
		return
	} else {
		s.verifySuccess(c)
	}
	//做一个判断 添加到对应的channel里面以供使用
	if flag, err := c.ReadFlag(); err == nil {
		s.typeDeal(flag, c, id)
	} else {
		logs.Warn(err, flag)
	}
	return
}

func (s *Bridge) closeClient(id int) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	if v, ok := s.Client[id]; ok {
		if c, err := file.GetCsvDb().GetClient(id); err == nil && c.NoStore {
			s.CloseClient <- c.Id
		}
		v.signal.WriteClose()
		delete(s.Client, id)
	}
}
func (s *Bridge) delClient(id int) {
	s.clientLock.Lock()
	defer s.clientLock.Unlock()
	if v, ok := s.Client[id]; ok {
		if c, err := file.GetCsvDb().GetClient(id); err == nil && c.NoStore {
			s.CloseClient <- c.Id
		}
		v.signal.Close()
		delete(s.Client, id)
	}
}

//tcp连接类型区分
func (s *Bridge) typeDeal(typeVal string, c *conn.Conn, id int) {
	switch typeVal {
	case common.WORK_MAIN:
		//客户端已经存在，下线
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
			s.Client[id] = NewClient(nil, c, nil)
			s.clientLock.Unlock()
		}
		logs.Info("clientId %d connection succeeded, address:%s ", id, c.Conn.RemoteAddr())
		go s.GetStatus(id)
	case common.WORK_CHAN:
		s.clientLock.Lock()
		if v, ok := s.Client[id]; ok {
			s.clientLock.Unlock()
			v.Lock()
			v.tunnel = c
			v.Unlock()
		} else {
			s.Client[id] = NewClient(c, nil, nil)
			s.clientLock.Unlock()
		}
		go s.clientCopy(id)
	case common.WORK_CONFIG:
		go s.GetConfig(c)
	case common.WORK_REGISTER:
		go s.register(c)
	case common.WORD_SECRET:
		if b, err := c.ReadLen(32); err == nil {
			s.SecretChan <- conn.NewSecret(string(b), c)
		}
	case common.WORK_SEND_STATUS:
		s.clientLock.Lock()
		if v, ok := s.Client[id]; ok {
			s.clientLock.Unlock()
			v.Lock()
			v.msg = c
			v.Unlock()
		} else {
			s.Client[id] = NewClient(nil, nil, c)
			s.clientLock.Unlock()
		}
		go s.getMsgStatus(id)
	}
	c.SetAlive(s.tunnelType)
	return
}

func (s *Bridge) getMsgStatus(clientId int) {
	s.clientLock.Lock()
	client := s.Client[clientId]
	s.clientLock.Unlock()

	if client == nil {
		return
	}
	for {
		if id, err := client.msg.GetLen(); err != nil {
			s.closeClient(clientId)
			return
		} else {
			client.Lock()
			if v, ok := client.linkMap[id]; ok {
				v.StatusCh <- true
			}
			client.Unlock()
		}
	}
}
func (s *Bridge) register(c *conn.Conn) {
	var hour int32
	if err := binary.Read(c, binary.LittleEndian, &hour); err == nil {
		s.registerLock.Lock()
		s.Register[common.GetIpByAddr(c.Conn.RemoteAddr().String())] = time.Now().Add(time.Hour * time.Duration(hour))
		s.registerLock.Unlock()
	}
}

//等待
func (s *Bridge) waitStatus(clientId, id int) bool {
	ticker := time.NewTicker(time.Millisecond * 100)
	stop := time.After(time.Second * 10)
	for {
		select {
		case <-ticker.C:
			s.clientLock.Lock()
			if v, ok := s.Client[clientId]; ok {
				s.clientLock.Unlock()
				v.Lock()
				if vv, ok := v.linkStatusMap[id]; ok {
					ticker.Stop()
					v.Unlock()
					return vv
				}
				v.Unlock()
			} else {
				s.clientLock.Unlock()
			}
		case <-stop:
			return false
		}
	}
}

func (s *Bridge) SendLinkInfo(clientId int, link *conn.Link, linkAddr string) (tunnel *conn.Conn, err error) {
	s.clientLock.Lock()
	if v, ok := s.Client[clientId]; ok {
		s.clientLock.Unlock()
		if s.ipVerify {
			s.registerLock.Lock()
			ip := common.GetIpByAddr(linkAddr)
			if v, ok := s.Register[ip]; !ok {
				s.registerLock.Unlock()
				return nil, errors.New(fmt.Sprintf("The ip %s is not in the validation list", ip))
			} else {
				if !v.After(time.Now()) {
					return nil, errors.New(fmt.Sprintf("The validity of the ip %s has expired", ip))
				}
			}
			s.registerLock.Unlock()
		}

		v.signal.SendLinkInfo(link)
		if err != nil {
			logs.Warn("send link information error:", err, link.Id)
			s.DelClient(clientId)
			return
		}

		if v.tunnel == nil {
			err = errors.New("get tunnel connection error")
			return
		} else {
			tunnel = v.tunnel
		}
		link.MsgConn = v.msg
		v.Lock()
		v.linkMap[link.Id] = link
		v.Unlock()
		if !s.waitStatus(clientId, link.Id) {
			err = errors.New(fmt.Sprintf("connect target %s fail", link.Host))
			return
		}
	} else {
		s.clientLock.Unlock()
		err = errors.New(fmt.Sprintf("the client %d is not connect", clientId))
	}
	return
}

//删除通信通道
func (s *Bridge) DelClient(id int) {
	s.closeClient(id)
}

//get config
func (s *Bridge) GetConfig(c *conn.Conn) {
	var client *file.Client
	var fail bool

	for {
		flag, err := c.ReadFlag()
		if err != nil {
			break
		}
		switch flag {
		case common.WORK_STATUS:
			if b, err := c.ReadLen(16); err != nil {
				break
			} else {
				var str string
				id, err := file.GetCsvDb().GetClientIdByVkey(string(b))
				if err != nil {
					break
				}
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
				binary.Write(c, binary.LittleEndian, int32(len([]byte(str))))
				binary.Write(c, binary.LittleEndian, []byte(str))
			}
		case common.NEW_CONF:
			var err error
			if client, err = c.GetConfigInfo(); err != nil {
				fail = true
				c.WriteAddFail()
				break
			} else {
				if err = file.GetCsvDb().NewClient(client); err != nil {
					fail = true
					c.WriteAddFail()
					break
				}
				c.WriteAddOk()
				c.Write([]byte(client.VerifyKey))
			}
		case common.NEW_HOST:
			if h, err := c.GetHostInfo(); err != nil {
				fail = true
				c.WriteAddFail()
				break
			} else if file.GetCsvDb().IsHostExist(h) {
				fail = true
				c.WriteAddFail()
				break
			} else {
				h.Client = client
				file.GetCsvDb().NewHost(h)
				c.WriteAddOk()
			}
		case common.NEW_TASK:
			if t, err := c.GetTaskInfo(); err != nil {
				fail = true
				c.WriteAddFail()
				break
			} else {
				ports := common.GetPorts(t.Ports)
				targets := common.GetPorts(t.Target)
				if len(ports) > 1 && (t.Mode == "tcpServer" || t.Mode == "udpServer") && (len(ports) != len(targets)) {
					fail = true
					c.WriteAddFail()
					break
				} else if t.Mode == "secretServer" {
					ports = append(ports, 0)
				}
				if len(ports) == 0 {
					fail = true
					c.WriteAddFail()
					break
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
						tl.Target = t.TargetAddr + ":" + strconv.Itoa(targets[i])
					}
					tl.Id = file.GetCsvDb().GetTaskId()
					tl.Status = true
					tl.Flow = new(file.Flow)
					tl.NoStore = true
					tl.Client = client
					tl.Password = t.Password
					if err := file.GetCsvDb().NewTask(tl); err != nil {
						logs.Notice("Add task error ", err.Error())
						fail = true
						c.WriteAddFail()
						break
					}
					if b := tool.TestServerPort(tl.Port, tl.Mode); !b && t.Mode != "secretServer" {
						fail = true
						c.WriteAddFail()
						break
					} else {
						s.OpenTask <- tl
					}
					c.WriteAddOk()
				}
			}
		}
	}
	if fail && client != nil {
		s.CloseClient <- client.Id
	}
	c.Close()
}

func (s *Bridge) GetStatus(clientId int) {
	s.clientLock.Lock()
	client := s.Client[clientId]
	s.clientLock.Unlock()

	if client == nil {
		return
	}
	for {
		if id, status, err := client.signal.GetConnStatus(); err != nil {
			s.closeClient(clientId)
			return
		} else {
			client.Lock()
			client.linkStatusMap[id] = status
			client.Unlock()
		}
	}
}

func (s *Bridge) clientCopy(clientId int) {

	s.clientLock.Lock()
	client := s.Client[clientId]
	s.clientLock.Unlock()

	for {
		if id, err := client.tunnel.GetLen(); err != nil {
			logs.Info("read msg content length error close client")
			s.delClient(clientId)
			break
		} else {
			client.Lock()
			if link, ok := client.linkMap[id]; ok {
				client.Unlock()
				if content, err := client.tunnel.GetMsgContent(link); err != nil {
					pool.PutBufPoolCopy(content)
					s.delClient(clientId)
					logs.Notice("read msg content error", err, "close client")
					break
				} else {
					link.MsgCh <- content
				}
			} else {
				client.Unlock()
				continue
			}
		}
	}
}

func (s *Bridge) linkCleanSession() {
	ticker := time.NewTicker(time.Minute * 5)
	for {
		select {
		case <-ticker.C:
			s.clientLock.Lock()
			for _, v := range s.Client {
				v.Lock()
				for _, vv := range v.linkMap {
					if vv.FinishUse {
						delete(v.linkMap, vv.Id)
					}
				}
				v.Unlock()
			}
			s.clientLock.Unlock()
		}
	}
}
