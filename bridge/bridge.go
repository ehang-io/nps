package bridge

import (
	"errors"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net"
	"strconv"
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

type Bridge struct {
	TunnelPort int                 //通信隧道端口
	listener   *net.TCPListener    //server端监听
	SignalList map[int]*list       //通信
	TunnelList map[int]*list       //隧道
	RunList    map[int]interface{} //运行中的任务
	lock       sync.Mutex
	tunnelLock sync.Mutex
}

func NewTunnel(tunnelPort int, runList map[int]interface{}) *Bridge {
	t := new(Bridge)
	t.TunnelPort = tunnelPort
	t.SignalList = make(map[int]*list)
	t.TunnelList = make(map[int]*list)
	t.RunList = runList
	return t
}

func (s *Bridge) StartTunnel() error {
	var err error
	s.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.TunnelPort, ""})
	if err != nil {
		return err
	}
	go s.tunnelProcess()
	return nil
}

//tcp server
func (s *Bridge) tunnelProcess() error {
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
func (s *Bridge) verifyError(c *utils.Conn) {
	c.Conn.Write([]byte(utils.VERIFY_EER))
	c.Conn.Close()
}

func (s *Bridge) cliProcess(c *utils.Conn) error {
	c.Conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	vval := make([]byte, 32)
	if _, err := c.Conn.Read(vval); err != nil {
		log.Println("客户端读超时。客户端地址为：:", c.Conn.RemoteAddr())
		c.Conn.Close()
		return err
	}
	id, err := utils.GetCsvDb().GetIdByVerifyKey(string(vval),c.Conn.RemoteAddr().String())
	if err != nil {
		log.Println("当前客户端连接校验错误，关闭此客户端:", c.Conn.RemoteAddr())
		s.verifyError(c)
		return errors.New("验证错误")
	}
	c.Conn.(*net.TCPConn).SetReadDeadline(time.Time{})
	//做一个判断 添加到对应的channel里面以供使用
	if flag, err := c.ReadFlag(); err != nil {
		return err
	} else {
		return s.typeDeal(flag, c, id)
	}
}

//tcp连接类型区分
func (s *Bridge) typeDeal(typeVal string, c *utils.Conn, id int) error {
	switch typeVal {
	case utils.WORK_MAIN:
		log.Println("客户端连接成功", c.Conn.RemoteAddr())
		s.addList(s.SignalList, c, id)
	case utils.WORK_CHAN:
		s.addList(s.TunnelList, c, id)
	default:
		return errors.New("无法识别")
	}
	c.SetAlive()
	return nil
}

//加到对应的list中
func (s *Bridge) addList(m map[int]*list, c *utils.Conn, id int) {
	s.lock.Lock()
	if v, ok := m[id]; ok {
		v.Add(c)
	} else {
		l := newList()
		l.Add(c)
		m[id] = l
	}
	s.lock.Unlock()
}

//新建隧道
func (s *Bridge) newChan(id int) error {
	var connPass *utils.Conn
	var err error
retry:
	if connPass, err = s.waitAndPop(s.SignalList, id); err != nil {
		return err
	}
	if _, err = connPass.Conn.Write([]byte("chan")); err != nil {
		goto retry
	}
	s.SignalList[id].Add(connPass)
	return nil
}

//得到一个tcp隧道
//TODO 超时问题 锁机制问题 对单个客户端加锁
func (s *Bridge) GetTunnel(id int, en, de int, crypt, mux bool) (c *utils.Conn, err error) {
retry:
	if c, err = s.waitAndPop(s.TunnelList, id); err != nil {
		return
	}
	if _, err = c.WriteTest(); err != nil {
		c.Close()
		goto retry
	}
	c.WriteConnInfo(en, de, crypt, mux)
	return
}

//得到一个通信通道
func (s *Bridge) GetSignal(id int) (err error, conn *utils.Conn) {
	if v, ok := s.SignalList[id]; !ok || v.Len() == 0 {
		err = errors.New("客户端未连接")
		return
	}
	conn = s.SignalList[id].Pop()
	return
}

//重回slice 复用
func (s *Bridge) ReturnSignal(conn *utils.Conn, id int) {
	if v, ok := s.SignalList[id]; ok {
		v.Add(conn)
	}
}

//重回slice 复用
func (s *Bridge) ReturnTunnel(conn *utils.Conn, id int) {
	if v, ok := s.TunnelList[id]; ok {
		utils.FlushConn(conn.Conn)
		v.Add(conn)
	}
}

//删除通信通道
func (s *Bridge) DelClientSignal(id int) {
	s.delClient(id, s.SignalList)
}

//删除隧道
func (s *Bridge) DelClientTunnel(id int) {
	s.delClient(id, s.TunnelList)
}

func (s *Bridge) delClient(id int, l map[int]*list) {
	if t := l[id]; t != nil {
		for {
			if t.Len() <= 0 {
				break
			}
			t.Pop().Close()
		}
		delete(l, id)
	}
}

//等待
func (s *Bridge) waitAndPop(m map[int]*list, id int) (c *utils.Conn, err error) {
	ticker := time.NewTicker(time.Millisecond * 100)
	stop := time.After(time.Second * 3)
	for {
		select {
		case <-ticker.C:
			s.lock.Lock()
			if v, ok := m[id]; ok && v.Len() > 0 {
				c = v.Pop()
				ticker.Stop()
				s.lock.Unlock()
				return
			}
			s.lock.Unlock()
		case <-stop:
			err = errors.New("client id: " + strconv.Itoa(id) + ",err: get client conn timeout")
			return
		}
	}
	return
}

func (s *Bridge) verify(id int) bool {
	for k := range s.RunList {
		if k == id {
			return true
		}
	}
	return false
}
