package utils

import (
	"net"
	"sync"
)

type Link struct {
	Id            int    //id
	ConnType      string //连接类型
	Host          string //目标
	En            int    //加密
	De            int    //解密
	Crypt         bool   //加密
	Conn          *Conn
	Flow          *Flow
	UdpListener   *net.UDPConn
	Rate          *Rate
	UdpRemoteAddr *net.UDPAddr
}

func NewLink(id int, connType string, host string, en, de int, crypt bool, conn *Conn, flow *Flow, udpListener *net.UDPConn, rate *Rate, UdpRemoteAddr *net.UDPAddr) *Link {
	return &Link{
		Id:            id,
		ConnType:      connType,
		Host:          host,
		En:            en,
		De:            de,
		Crypt:         crypt,
		Conn:          conn,
		Flow:          flow,
		UdpListener:   udpListener,
		Rate:          rate,
		UdpRemoteAddr: UdpRemoteAddr,
	}
}

type Flow struct {
	ExportFlow int64 //出口流量
	InletFlow  int64 //入口流量
	FlowLimit  int64 //流量限制，出口+入口 /M
	sync.RWMutex
}

func (s *Flow) Add(in, out int) {
	s.Lock()
	defer s.Unlock()
	s.InletFlow += int64(in)
	s.ExportFlow += int64(out)
}

type Client struct {
	Cnf       *Config
	Id        int    //id
	VerifyKey string //验证密钥
	Addr      string //客户端ip地址
	Remark    string //备注
	Status    bool   //是否开启
	IsConnect bool   //是否连接
	RateLimit int    //速度限制 /kb
	Flow      *Flow  //流量
	Rate      *Rate  //速度控制
	id        int
	sync.RWMutex
}

func (s *Client) GetId() int {
	s.Lock()
	defer s.Unlock()
	s.id++
	return s.id
}

type Tunnel struct {
	Id           int     //Id
	TcpPort      int     //服务端与客户端通信端口
	Mode         string  //启动方式
	Target       string  //目标
	Status       bool    //是否开启
	Client       *Client //所属客户端id
	Flow         *Flow
	Config       *Config
	UseClientCnf bool   //是否继承客户端配置
	Remark       string //备注
}

type Config struct {
	U              string //socks5验证用户名
	P              string //socks5验证密码
	Compress       string //压缩方式
	Crypt          bool   //是否加密
	CompressEncode int    //加密方式
	CompressDecode int    //解密方式
}

type Host struct {
	Host         string //启动方式
	Target       string //目标
	HeaderChange string //host修改
	HostChange   string //host修改
	Flow         *Flow
	Client       *Client
	Remark       string //备注
}

//深拷贝Config
func DeepCopyConfig(c *Config) *Config {
	return &Config{
		U:              c.U,
		P:              c.P,
		Compress:       c.Compress,
		Crypt:          c.Crypt,
		CompressEncode: c.CompressEncode,
		CompressDecode: c.CompressDecode,
	}
}
