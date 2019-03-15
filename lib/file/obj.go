package file

import (
	"github.com/cnlh/nps/lib/rate"
	"github.com/pkg/errors"
	"strings"
	"sync"
	"time"
)

type Flow struct {
	ExportFlow int64 //出口流量
	InletFlow  int64 //入口流量
	FlowLimit  int64 //流量限制，出口+入口 /M
	sync.RWMutex
}

func (s *Flow) Add(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.InletFlow += int64(in)
	s.ExportFlow += int64(out)
}

type Client struct {
	Cnf       *Config
	Id        int        //id
	VerifyKey string     //验证密钥
	Addr      string     //客户端ip地址
	Remark    string     //备注
	Status    bool       //是否开启
	IsConnect bool       //是否连接
	RateLimit int        //速度限制 /kb
	Flow      *Flow      //流量
	Rate      *rate.Rate //速度控制
	NoStore   bool
	NoDisplay bool
	MaxConn   int //客户端最大连接数
	NowConn   int //当前连接数
	id        int
	sync.RWMutex
}

func NewClient(vKey string, noStore bool, noDisplay bool) *Client {
	return &Client{
		Cnf:       new(Config),
		Id:        0,
		VerifyKey: vKey,
		Addr:      "",
		Remark:    "",
		Status:    true,
		IsConnect: false,
		RateLimit: 0,
		Flow:      new(Flow),
		Rate:      nil,
		NoStore:   noStore,
		RWMutex:   sync.RWMutex{},
		NoDisplay: noDisplay,
	}
}

func (s *Client) CutConn() {
	s.Lock()
	defer s.Unlock()
	s.NowConn++
}

func (s *Client) AddConn() {
	s.Lock()
	defer s.Unlock()
	s.NowConn--
}

func (s *Client) GetConn() bool {
	if s.MaxConn == 0 || s.NowConn < s.MaxConn {
		s.CutConn()
		return true
	}
	return false
}

//modify the hosts and the tunnels by health information
func (s *Client) ModifyTarget() {

}

func (s *Client) HasTunnel(t *Tunnel) bool {
	GetCsvDb().Lock()
	defer GetCsvDb().Unlock()
	for _, v := range GetCsvDb().Tasks {
		if v.Client.Id == s.Id && v.Port == t.Port {
			return true
		}
	}
	return false
}

func (s *Client) HasHost(h *Host) bool {
	GetCsvDb().Lock()
	defer GetCsvDb().Unlock()
	for _, v := range GetCsvDb().Hosts {
		if v.Client.Id == s.Id && v.Host == h.Host && h.Location == v.Location {
			return true
		}
	}
	return false
}

type Tunnel struct {
	Id         int      //Id
	Port       int      //服务端监听端口
	Mode       string   //启动方式
	Target     string   //目标
	TargetArr  []string //目标
	Status     bool     //设置是否开启
	RunStatus  bool     //当前运行状态
	Client     *Client  //所属客户端id
	Ports      string   //客户端与服务端传递
	Flow       *Flow
	Password   string //私密模式密码，唯一
	Remark     string //备注
	TargetAddr string
	NoStore    bool
	LocalPath  string
	StripPre   string
	NowIndex   int
	Health
	sync.RWMutex
}

type Health struct {
	HealthCheckTimeout  int
	HealthMaxFail       int
	HealthCheckInterval int
	HealthNextTime      time.Time
	HealthMap           map[string]int
	HttpHealthUrl       string
	HealthRemoveArr     []string
	HealthCheckType     string
	HealthCheckTarget   string
}

func (s *Tunnel) GetRandomTarget() (string, error) {
	if s.TargetArr == nil {
		s.TargetArr = strings.Split(s.Target, "\n")
	}
	if len(s.TargetArr) == 1 {
		return s.TargetArr[0], nil
	}
	if len(s.TargetArr) == 0 {
		return "", errors.New("all inward-bending targets are offline")
	}
	s.Lock()
	defer s.Unlock()
	if s.NowIndex >= len(s.TargetArr)-1 {
		s.NowIndex = -1
	}
	s.NowIndex++
	return s.TargetArr[s.NowIndex], nil
}

type Config struct {
	U        string //socks5验证用户名
	P        string //socks5验证密码
	Compress bool   //压缩方式
	Crypt    bool   //是否加密
}

type Host struct {
	Id           int
	Host         string //启动方式
	Target       string //目标
	HeaderChange string //host修改
	HostChange   string //host修改
	Location     string //url 路由
	Flow         *Flow
	Client       *Client
	Remark       string //备注
	NowIndex     int
	TargetArr    []string
	NoStore      bool
	Scheme       string //http https all
	IsClose      bool
	Health
	sync.RWMutex
}

func (s *Host) GetRandomTarget() (string, error) {
	if s.TargetArr == nil {
		s.TargetArr = strings.Split(s.Target, "\n")
	}
	if len(s.TargetArr) == 1 {
		return s.TargetArr[0], nil
	}
	if len(s.TargetArr) == 0 {
		return "", errors.New("all inward-bending targets are offline")
	}
	s.Lock()
	defer s.Unlock()
	if s.NowIndex >= len(s.TargetArr)-1 {
		s.NowIndex = -1
	}
	s.NowIndex++
	return s.TargetArr[s.NowIndex], nil
}
