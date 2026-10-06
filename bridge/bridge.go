package bridge

import (
	"crypto/tls"
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

	"ehang.io/nps/lib/bridgetls"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/selfupdate"
	"ehang.io/nps/lib/sysstat"
	"ehang.io/nps/lib/version"
	"ehang.io/nps/server/connection"
	"ehang.io/nps/server/tool"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type Client struct {
	tunnel *nps_mux.Mux
	signal *conn.Conn
	// signalMu serialises writes to signal. Messages on it are multi-part --
	// a four byte flag followed by length-prefixed frames -- and there is
	// more than one writer: the per-connection goroutine handling p2p, and
	// the web handler pushing updates. Two interleaved sequences leave the
	// client reading spliced flags and lengths, and its control channel
	// stays broken until it reconnects.
	signalMu  sync.Mutex
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
	stats          sync.Map // client id -> *sysstat.History
	Register       sync.Map
	tunnelType     string //bridge type kcp or tcp
	OpenTask       chan *file.Tunnel
	CloseTask      chan *file.Tunnel
	CloseClient    chan int
	SecretChan     chan *conn.Secret
	ipVerify       bool
	runList        sync.Map //map[int]interface{}
	disconnectTime int
	tlsConfig      *tls.Config
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
	tlsConfig, err := bridgetls.ServerConfig()
	if err != nil {
		return fmt.Errorf("bridge TLS: %w", err)
	}
	s.tlsConfig = tlsConfig
	go s.ping()
	if s.tunnelType == "kcp" {
		logs.Info("server start, the bridge type is %s, the bridge port is %d", s.tunnelType, s.TunnelPort)
		return conn.NewKcpListenerAndProcess(beego.AppConfig.String("bridge_ip")+":"+beego.AppConfig.String("bridge_port"), s.serve)
	}
	listener, err := connection.GetBridgeListener(s.tunnelType)
	if err != nil {
		logs.Error(err)
		os.Exit(0)
		return err
	}
	conn.Accept(listener, s.serve)
	return nil
}

// serve secures a freshly accepted connection and hands it on.
func (s *Bridge) serve(c net.Conn) {
	t, err := bridgetls.Server(c, s.tlsConfig)
	if err != nil {
		logs.Info("TLS handshake with %s failed: %s", c.RemoteAddr(), err.Error())
		return
	}
	s.cliProcess(conn.NewConn(t))
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
	// Tear the client down only if this is still its control connection.
	// After a reconnect the entry holds the new session, and the old
	// connection's read failing must not close the new signal and muxes.
	if v, ok := s.Client.Load(id); ok && v.(*Client).signal != c {
		_ = c.Close()
		return
	}
	s.DelClient(id)
}

// cliProcess reads the hello of a connection that is already TLS: the protocol
// revision, the client's version, what the connection is for, and a proof that
// the client holds the vkey of one of the clients here. The proof is an HMAC
// bound to this TLS session, see bridgetls, so it names no key and cannot be
// carried to another session. The answer is a verdict and, on success, the
// server's own proof, which is how the client learns that it is talking to the
// server and not to something in between. Nothing the client does next is
// sent before it has checked it.
func (s *Bridge) cliProcess(c *conn.Conn) {
	ip := common.GetIpByAddr(c.Conn.RemoteAddr().String())
	c.SetReadDeadlineBySecond(10)
	protocol, err := c.GetShortLenContent()
	if err != nil {
		logs.Info("The client %s connect error %s", c.Conn.RemoteAddr(), err.Error())
		c.Close()
		return
	}
	if string(protocol) != version.Protocol {
		// Judged before anything else is read: a client of another revision may
		// lay out the rest of its hello differently, and would leave this
		// reading until the deadline instead of hearing why.
		logs.Info("The client %s speaks protocol %q, this server %q", c.Conn.RemoteAddr(), protocol, version.Protocol)
		c.Write([]byte(common.VERIFY_PROTOCOL))
		c.Close()
		return
	}
	vs, err := c.GetShortLenContent()
	if err != nil {
		logs.Info("get client %s version error %s", c.Conn.RemoteAddr(), err.Error())
		c.Close()
		return
	}
	flag, err := c.ReadFlag()
	if err != nil {
		logs.Warn(err, flag)
		c.Close()
		return
	}
	proof, err := c.GetShortContent(bridgetls.ProofLen)
	if err != nil {
		c.Close()
		return
	}
	exporter, err := bridgetls.Exporter(c.Conn)
	if err != nil {
		logs.Warn(err)
		c.Close()
		return
	}
	hello := bridgetls.Hello(string(protocol), string(vs), flag)
	id, vkey, err := file.GetDb().FindClientByProof(func(k string) bool {
		return bridgetls.Equal(bridgetls.ClientProof(k, exporter, hello), proof)
	}, c.Conn.RemoteAddr().String())
	if err != nil {
		raise("unknown-key", ip, 0, "a connection proved no vkey that any enabled client here holds: a wrong or disabled key, a scanner, or someone relaying a client's session")
		c.Write([]byte(common.VERIFY_EER))
		c.Close()
		return
	}
	c.Write(append([]byte(common.VERIFY_SUCCESS), bridgetls.ServerProof(vkey, exporter, proof)...))
	warnWeakVkey(id, vkey, ip)
	c.SetAlive()
	s.typeDeal(flag, c, id, string(vs))
}

// SendUpdate asks a connected client to replace its own binary. Only the
// request and the tag travel down the control connection: the node fetches
// the release from GitHub itself, so the server never has to hold artifacts
// for every platform it serves, and a twelve megabyte binary never competes
// for bandwidth with the traffic the tunnel exists to carry.
//
// The reply comes as a reconnect. A node that comes back with a new Version
// updated successfully; one that comes back with the old Version rolled
// itself back; one that does not come back needs a look.
func (s *Bridge) SendUpdate(id int, tag string) error {
	if !selfupdate.ValidTag(tag) {
		return errors.New("that is not a valid release tag")
	}
	v, ok := s.Client.Load(id)
	if !ok {
		return errors.New("the client is not connected")
	}
	c := v.(*Client)
	if c.signal == nil {
		return errors.New("the client has no control connection")
	}
	c.signalMu.Lock()
	defer c.signalMu.Unlock()
	if _, err := c.signal.Write([]byte(common.WORK_UPDATE)); err != nil {
		return err
	}
	// The tag is always written, empty meaning the latest release.
	return c.signal.WriteLenContent([]byte(tag))
}

// statsHistory is the host stats history of a client. It outlives the
// client's connection, so a reconnect keeps what came before.
func (s *Bridge) statsHistory(id int) *sysstat.History {
	h, _ := s.stats.LoadOrStore(id, new(sysstat.History))
	return h.(*sysstat.History)
}

// ClientStats returns the host stats a client has reported in the last
// sysstat.Retention, oldest first; decode each record with sysstat.Decode.
func (s *Bridge) ClientStats(id int) []sysstat.Point {
	if h, ok := s.stats.Load(id); ok {
		return h.(*sysstat.History).Points(time.Now())
	}
	return nil
}

// ForgetStats drops a client's stats history. It is for a client that is
// gone for good: a disconnect keeps the history, so a reconnect continues it.
func (s *Bridge) ForgetStats(id int) {
	s.stats.Delete(id)
}

// ClientLastStats is the newest host stats record a client has reported.
func (s *Bridge) ClientLastStats(id int) (sysstat.Point, bool) {
	if h, ok := s.stats.Load(id); ok {
		return h.(*sysstat.History).Last()
	}
	return sysstat.Point{}, false
}

func (s *Bridge) DelClient(id int) {
	if v, ok := s.Client.Load(id); ok {
		if v.(*Client).signal != nil {
			v.(*Client).signal.Close()
		}
		// The muxes carry every open connection through this client. Left
		// open they keep their sockets, goroutines and queued data until
		// their own pings give up, minutes later.
		if v.(*Client).tunnel != nil {
			_ = v.(*Client).tunnel.Close()
		}
		if v.(*Client).file != nil {
			_ = v.(*Client).file.Close()
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
		tcpConn, ok := bridgetls.TCPConn(c.Conn)
		if ok {
			// add tcp keep alive option for signal connection
			_ = tcpConn.SetKeepAlive(true)
			_ = tcpConn.SetKeepAlivePeriod(5 * time.Second)
		}
		//the vKey connect by another ,close the client of before
		if v, ok := s.Client.LoadOrStore(id, NewClient(nil, nil, c, vs)); ok {
			if old := v.(*Client).signal; old != nil {
				// the same vkey from another address while the first is still
				// here is a client started twice, or a key that has got out
				if oldIP, newIP := common.GetIpByAddr(old.Conn.RemoteAddr().String()), common.GetIpByAddr(c.Conn.RemoteAddr().String()); oldIP != newIP {
					raise("duplicate", newIP, id, fmt.Sprintf("client %d connected from %s while still connected from %s; the first is dropped. A client that changed network and has not been noticed gone looks the same, as does a client started twice or a vkey that got out", id, newIP, oldIP))
				}
				old.WriteClose()
			}
			v.(*Client).signal = c
			v.(*Client).Version = vs
		}
		go s.GetHealthFromClient(id, c)
		logs.Info("clientId %d connection succeeded, address:%s ", id, c.Conn.RemoteAddr())
	case common.WORK_CHAN:
		hist := s.statsHistory(id)
		muxConn := nps_mux.NewMuxStats(c.Conn, s.tunnelType, s.disconnectTime, nil, func(rec []byte) {
			hist.Add(time.Now(), rec)
		})
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
	case common.WORK_SECRET_MUX:
		// say that this server understands the type: a visitor does not use
		// the link until it has heard this, and an older server says nothing
		if _, err := c.Write([]byte(common.WORK_SECRET_MUX)); err != nil {
			c.Close()
			return
		}
		go s.serveSecretStreams(nps_mux.NewMuxLiveness(c.Conn, s.tunnelType, common.SecretLinkPingCheck, common.SecretLinkPingEvery))
	case common.WORK_SECRET:
		s.dealSecret(c)
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
				svrAddr := beego.AppConfig.String("p2p_ip") + ":" + beego.AppConfig.String("p2p_port")
				if err != nil {
					logs.Warn("get local udp addr error")
					return
				}
				//向密钥对应的客户端发送与服务端udp建立连接信息，地址，密钥
				//这三个写是一条消息，必须整体持锁：SendUpdate 也写同一个 signal
				target := v.(*Client)
				target.signalMu.Lock()
				target.signal.Write([]byte(common.NEW_UDP_CONN))
				target.signal.WriteLenContent([]byte(svrAddr))
				target.signal.WriteLenContent(b)
				target.signalMu.Unlock()
				//向该请求者发送建立连接请求,服务器地址
				c.WriteLenContent([]byte(svrAddr))
			}
		}
	}
	c.SetAlive()
	return
}

// serveSecretStreams takes the streams of a visitor's secret link. Each opens
// with the digest of a secret, which is then dealt with as the secret of a
// connection of its own is.
func (s *Bridge) serveSecretStreams(m *nps_mux.Mux) {
	for {
		st, err := m.Accept()
		if err != nil {
			return
		}
		go s.dealSecret(conn.NewConn(st))
	}
}

// dealSecret reads the digest of a secret, which is what a connection or a
// stream for a secret tunnel opens with, and passes the connection on to the
// task that has the secret. One that does not say it in good time is dropped.
func (s *Bridge) dealSecret(c *conn.Conn) {
	c.SetReadDeadlineBySecond(10)
	b, err := c.GetShortContent(32)
	if err != nil {
		logs.Error("secret error, failed to match the key successfully")
		c.Close()
		return
	}
	c.SetAlive()
	s.SecretChan <- conn.NewSecret(string(b), c)
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

// get config and add task from client config
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
					if own := client.OwnTunnel(tl); own != nil {
						// The client is registering a key it already owns,
						// from a session the server has not seen end or one
						// whose reply was lost. Take the target from this
						// registration, so an edited npc.conf is not ignored
						// until the old tunnel happens to be cleaned up. A
						// tunnel made in the web UI keeps its own target.
						if own.IsKeyed() && own.NoStore {
							if old := own.GetTarget(); old == nil || old.TargetStr != tl.Target.TargetStr || old.LocalProxy != tl.Target.LocalProxy {
								logs.Info("task %s: target changed to %s on re-registration", own.Remark, tl.Target.TargetStr)
								own.SetTarget(tl.Target)
							}
						}
					} else {
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
