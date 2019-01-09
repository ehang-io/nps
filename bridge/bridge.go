package bridge

import (
	"errors"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"sync"
	"time"
)

type list struct {
	connList chan *utils.Conn
}

func (l *list) Add(c *utils.Conn) {
	l.connList <- c
}

func (l *list) Pop() *utils.Conn {
	return <-l.connList
}
func (l *list) Len() int {
	return len(l.connList)
}

func newList() *list {
	l := new(list)
	l.connList = make(chan *utils.Conn, 1000)
	return l
}

type Tunnel struct {
	TunnelPort int                    //通信隧道端口
	listener   *net.TCPListener       //server端监听
	SignalList map[string]*list       //通信
	TunnelList map[string]*list       //隧道
	RunList    map[string]interface{} //运行中的任务
	lock       sync.Mutex
	tunnelLock sync.Mutex
}

func NewTunnel(tunnelPort int, runList map[string]interface{}) *Tunnel {
	t := new(Tunnel)
	t.TunnelPort = tunnelPort
	t.SignalList = make(map[string]*list)
	t.TunnelList = make(map[string]*list)
	t.RunList = runList
	return t
}

func (s *Tunnel) StartTunnel() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.TunnelPort, ""})
	if err != nil {
		return err
	}
	go s.tunnelProcess()
	return nil
}

//tcp server
func (s *Tunnel) tunnelProcess() error {
	var err error
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.cliProcess(utils.NewConn(conn))
	}
	return err
}

//验证失败，返回错误验证flag，并且关闭连接
func (s *Tunnel) verifyError(c *utils.Conn) {
	c.Conn.Write([]byte(utils.VERIFY_EER))
	c.Conn.Close()
}

func (s *Tunnel) cliProcess(c *utils.Conn) error {
	c.Conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	vval := make([]byte, 32)
	if _, err := c.Conn.Read(vval); err != nil {
		log.Println("客户端读超时。客户端地址为：:", c.Conn.RemoteAddr())
		c.Conn.Close()
		return err
	}
	if !s.verify(string(vval)) {
		log.Println("当前客户端连接校验错误，关闭此客户端:", c.Conn.RemoteAddr())
		s.verifyError(c)
		return errors.New("验证错误")
	}
	log.Println("客户端连接成功: ", c.Conn.RemoteAddr())
	c.Conn.(*net.TCPConn).SetReadDeadline(time.Time{})
	//做一个判断 添加到对应的channel里面以供使用
	if flag, err := c.ReadFlag(); err != nil {
		return err
	} else {
		return s.typeDeal(flag, c, string(vval))
	}
}

//tcp连接类型区分
func (s *Tunnel) typeDeal(typeVal string, c *utils.Conn, cFlag string) error {
	switch typeVal {
	case utils.WORK_MAIN:
		s.addList(s.SignalList, c, cFlag)
	case utils.WORK_CHAN:
		s.addList(s.TunnelList, c, cFlag)
	default:
		return errors.New("无法识别")
	}
	c.SetAlive()
	return nil
}

//加到对应的list中
func (s *Tunnel) addList(m map[string]*list, c *utils.Conn, cFlag string) {
	s.lock.Lock()
	if v, ok := m[cFlag]; ok {
		v.Add(c)
	} else {
		l := newList()
		l.Add(c)
		m[cFlag] = l
	}
	s.lock.Unlock()
}

//新建隧道
func (s *Tunnel) newChan(cFlag string) error {
	if err := s.wait(s.SignalList, cFlag); err != nil {
		return err
	}
retry:
	connPass := s.SignalList[cFlag].Pop()
	_, err := connPass.Conn.Write([]byte("chan"))
	if err != nil {
		log.Println(err)
		goto retry
	}
	s.SignalList[cFlag].Add(connPass)
	return nil
}

//得到一个tcp隧道
func (s *Tunnel) GetTunnel(cFlag string, en, de int, crypt, mux bool) (c *utils.Conn, err error) {
	s.tunnelLock.Lock()
	if v, ok := s.TunnelList[cFlag]; !ok || v.Len() < 3 { //新建通道
		go s.newChan(cFlag)
	}
retry:
	if err = s.wait(s.TunnelList, cFlag); err != nil {
		return
	}
	c = s.TunnelList[cFlag].Pop()
	if _, err = c.WriteTest(); err != nil {
		c.Close()
		goto retry
	}
	c.WriteConnInfo(en, de, crypt, mux)
	s.tunnelLock.Unlock()
	return
}

//得到一个通信通道
func (s *Tunnel) GetSignal(cFlag string) (err error, conn *utils.Conn) {
	if v, ok := s.SignalList[cFlag]; !ok || v.Len() == 0 {
		err = errors.New("客户端未连接")
		return
	}
	conn = s.SignalList[cFlag].Pop()
	return
}

//重回slice 复用
func (s *Tunnel) ReturnSignal(conn *utils.Conn, cFlag string) {
	if v, ok := s.SignalList[cFlag]; ok {
		v.Add(conn)
	}
}

//重回slice 复用
func (s *Tunnel) ReturnTunnel(conn *utils.Conn, cFlag string) {
	if v, ok := s.TunnelList[cFlag]; ok {
		utils.FlushConn(conn.Conn)
		v.Add(conn)
	}
}

//删除通信通道
func (s *Tunnel) DelClientSignal(cFlag string) {
	s.delClient(cFlag, s.SignalList)
}

//删除隧道
func (s *Tunnel) DelClientTunnel(cFlag string) {
	s.delClient(cFlag, s.TunnelList)
}

func (s *Tunnel) delClient(cFlag string, l map[string]*list) {
	if t := l[utils.Getverifyval(cFlag)]; t != nil {
		for {
			if t.Len() <= 0 {
				break
			}
			t.Pop().Close()
		}
		delete(l, utils.Getverifyval(cFlag))
	}
}

//等待
func (s *Tunnel) wait(m map[string]*list, cFlag string) error {
	ticker := time.NewTicker(time.Millisecond * 100)
	stop := time.After(time.Second * 10)
loop:
	for {
		select {
		case <-ticker.C:
			if _, ok := m[cFlag]; ok {
				ticker.Stop()
				break loop
			}
		case <-stop:
			return errors.New("client key: " + cFlag + ",err: get client conn timeout")
		}
	}
	return nil
}

func (s *Tunnel) verify(vKeyMd5 string) bool {
	for k := range s.RunList {
		if utils.Getverifyval(k) == vKeyMd5 {
			return true
		}
	}
	return false
}
