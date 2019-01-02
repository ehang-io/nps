package lib

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type list struct {
	connList chan *Conn
}

func (l *list) Add(c *Conn) {
	l.connList <- c
}

func (l *list) Pop() *Conn {
	return <-l.connList
}
func (l *list) Len() int {
	return len(l.connList)
}

func newList() *list {
	l := new(list)
	l.connList = make(chan *Conn, 100)
	return l
}

type Tunnel struct {
	tunnelPort int              //通信隧道端口
	listener   *net.TCPListener //server端监听
	signalList map[string]*list //通信
	tunnelList map[string]*list //隧道
	sync.Mutex
}

func newTunnel(tunnelPort int) *Tunnel {
	t := new(Tunnel)
	t.tunnelPort = tunnelPort
	t.signalList = make(map[string]*list)
	t.tunnelList = make(map[string]*list)
	return t
}

func (s *Tunnel) StartTunnel() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.tunnelPort, ""})
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
		go s.cliProcess(NewConn(conn))
	}
	return err
}

//验证失败，返回错误验证flag，并且关闭连接
func (s *Tunnel) verifyError(c *Conn) {
	c.conn.Write([]byte(VERIFY_EER))
	c.conn.Close()
}

func (s *Tunnel) cliProcess(c *Conn) error {
	c.conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	vval := make([]byte, 32)
	if _, err := c.conn.Read(vval); err != nil {
		log.Println("客户端读超时。客户端地址为：:", c.conn.RemoteAddr())
		c.conn.Close()
		return err
	}
	if !verify(string(vval)) {
		log.Println("当前客户端连接校验错误，关闭此客户端:", c.conn.RemoteAddr())
		s.verifyError(c)
		return err
	}
	c.conn.(*net.TCPConn).SetReadDeadline(time.Time{})
	//做一个判断 添加到对应的channel里面以供使用
	if flag, err := c.ReadFlag(); err != nil {
		return err
	} else {
		return s.typeDeal(flag, c, string(vval))
	}
}

//tcp连接类型区分
func (s *Tunnel) typeDeal(typeVal string, c *Conn, cFlag string) error {
	switch typeVal {
	case WORK_MAIN:
		s.addList(s.signalList, c, cFlag)
	case WORK_CHAN:
		s.addList(s.tunnelList, c, cFlag)
	default:
		return errors.New("无法识别")
	}
	c.SetAlive()
	return nil
}

//加到对应的list中
func (s *Tunnel) addList(m map[string]*list, c *Conn, cFlag string) {
	s.Lock()
	if v, ok := m[cFlag]; ok {
		v.Add(c)
	} else {
		l := newList()
		l.Add(c)
		m[cFlag] = l
	}
	s.Unlock()
}

//新建隧道
func (s *Tunnel) newChan(cFlag string) error {
	if err := s.wait(s.signalList, cFlag); err != nil {
		return err
	}
retry:
	connPass := s.signalList[cFlag].Pop()
	_, err := connPass.conn.Write([]byte("chan"))
	if err != nil {
		log.Println(err)
		goto retry
	}
	s.signalList[cFlag].Add(connPass)
	return nil
}

//得到一个tcp隧道
func (s *Tunnel) GetTunnel(cFlag string, en, de int, crypt bool) (c *Conn, err error) {
	if v, ok := s.tunnelList[cFlag]; !ok || v.Len() < 10 { //新建通道
		go s.newChan(cFlag)
	}
retry:
	if err = s.wait(s.tunnelList, cFlag); err != nil {
		return
	}
	c = s.tunnelList[cFlag].Pop()
	if _, err := c.wTest(); err != nil {
		c.Close()
		goto retry
	}
	c.WriteConnInfo(en, de, crypt)
	return
}

//得到一个通信通道
func (s *Tunnel) GetSignal(cFlag string) (err error, conn *Conn) {
	if v, ok := s.signalList[cFlag]; !ok || v.Len() == 0 {
		err = errors.New("客户端未连接")
		return
	}
	conn = s.signalList[cFlag].Pop()
	return
}

//重回slice 复用
func (s *Tunnel) ReturnSignal(conn *Conn, cFlag string) {
	if v, ok := s.signalList[cFlag]; ok {
		v.Add(conn)
	}
}

//删除通信通道
func (s *Tunnel) DelClientSignal(cFlag string) {
	s.delClient(cFlag, s.signalList)
}

//删除隧道
func (s *Tunnel) DelClientTunnel(cFlag string) {
	s.delClient(cFlag, s.tunnelList)
}

func (s *Tunnel) delClient(cFlag string, l map[string]*list) {
	if t := l[getverifyval(cFlag)]; t != nil {
		for {
			if t.Len() <= 0 {
				break
			}
			t.Pop().Close()
		}
		delete(l, getverifyval(cFlag))
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
