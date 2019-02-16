package client

import (
	"errors"
	"github.com/cnlh/nps/lib/common"
	"github.com/cnlh/nps/lib/config"
	"github.com/cnlh/nps/lib/conn"
	"github.com/cnlh/nps/lib/lg"
	"github.com/cnlh/nps/lib/pool"
	"github.com/cnlh/nps/vender/github.com/xtaci/kcp"
	"github.com/cnlh/nps/vender/golang.org/x/net/proxy"
	"io/ioutil"
	"net"
	"net/url"
	"path/filepath"
	"sync"
	"time"
)

type TRPClient struct {
	svrAddr        string
	linkMap        map[int]*conn.Link
	tunnel         *conn.Conn
	bridgeConnType string
	stop           chan bool
	proxyUrl       string
	sync.Mutex
	vKey string
}

//new client
func NewRPClient(svraddr string, vKey string, bridgeConnType string, proxyUrl string) *TRPClient {
	return &TRPClient{
		svrAddr:        svraddr,
		linkMap:        make(map[int]*conn.Link),
		Mutex:          sync.Mutex{},
		vKey:           vKey,
		bridgeConnType: bridgeConnType,
		stop:           make(chan bool),
		proxyUrl:       proxyUrl,
	}
}

//start
func (s *TRPClient) Start() {
retry:
	c, err := NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_MAIN, s.proxyUrl)
	if err != nil {
		lg.Println("The connection server failed and will be reconnected in five seconds")
		time.Sleep(time.Second * 5)
		goto retry
	}
	lg.Printf("Successful connection with server %s", s.svrAddr)
	s.processor(c)
}

func (s *TRPClient) Close() {
	s.tunnel.Close()
	s.stop <- true
	for _, v := range s.linkMap {
		if v.Conn != nil {
			v.Conn.Close()
		}
	}
}

//处理
func (s *TRPClient) processor(c *conn.Conn) {
	go s.dealChan()
	for {
		flags, err := c.ReadFlag()
		if err != nil {
			lg.Printf("Accept server data error %s, end this service", err.Error())
			break
		}
		switch flags {
		case common.VERIFY_EER:
			lg.Fatalf("VKey:%s is incorrect, the server refuses to connect, please check", s.vKey)
		case common.NEW_CONN:
			if link, err := c.GetLinkInfo(); err != nil {
				break
			} else {
				s.Lock()
				s.linkMap[link.Id] = link
				s.Unlock()
				go s.linkProcess(link, c)
			}
		case common.RES_CLOSE:
			lg.Fatalln("The authentication key is connected by another client or the server closes the client.")
		case common.RES_MSG:
			lg.Println("Server-side return error")
			break
		default:
			lg.Println("The error could not be resolved")
			break
		}
	}
	c.Close()
	s.Close()
}

func (s *TRPClient) linkProcess(link *conn.Link, c *conn.Conn) {
	link.Host = common.FormatAddress(link.Host)
	//与目标建立连接
	server, err := net.DialTimeout(link.ConnType, link.Host, time.Second*3)

	if err != nil {
		c.WriteFail(link.Id)
		lg.Println("connect to ", link.Host, "error:", err)
		return
	}

	c.WriteSuccess(link.Id)

	link.Conn = conn.NewConn(server)
	buf := pool.BufPoolCopy.Get().([]byte)
	for {
		if n, err := server.Read(buf); err != nil {
			s.tunnel.SendMsg([]byte(common.IO_EOF), link)
			break
		} else {
			if _, err := s.tunnel.SendMsg(buf[:n], link); err != nil {
				c.Close()
				break
			}
		}
	}
	pool.PutBufPoolCopy(buf)
	s.Lock()
	delete(s.linkMap, link.Id)
	s.Unlock()
}

//隧道模式处理
func (s *TRPClient) dealChan() {
	var err error
	s.tunnel, err = NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_CHAN, s.proxyUrl)
	if err != nil {
		lg.Println("connect to ", s.svrAddr, "error:", err)
		return
	}

	go func() {
		for {
			if id, err := s.tunnel.GetLen(); err != nil {
				break
			} else {
				s.Lock()
				if v, ok := s.linkMap[id]; ok {
					s.Unlock()
					if content, err := s.tunnel.GetMsgContent(v); err != nil {
						lg.Println("get msg content error:", err, id)
						pool.PutBufPoolCopy(content)
						break
					} else {
						if len(content) == len(common.IO_EOF) && string(content) == common.IO_EOF {
							v.Conn.Close()
						} else if v.Conn != nil {
							v.Conn.Write(content)
						}
						pool.PutBufPoolCopy(content)
					}
				} else {
					s.Unlock()
				}
			}
		}
	}()
	<-s.stop
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
