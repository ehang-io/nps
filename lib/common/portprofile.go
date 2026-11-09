package common

import "time"

// PortProfile 描述一类「公网上被用得最多、也最容易被扫描」的服务端口在穿透时需要的参数。
//
// 内网穿透最常暴露的端口就那几个：3389(RDP)、22(SSH)、21(FTP)、80/443(HTTP)，
// 它们的握手特征各不相同，统一套一种连接策略一定会出问题：
//   - RDP/SSH/HTTP 是客户端先发数据，建立 TCP 后迟迟不发字节的连接基本都是扫描器
//   - FTP 反过来是服务器先发 220 欢迎语，等客户端首包会把正常用户挡掉
//   - RDP 断线时对端已经消失但连接没关，靠 TCP 重传要等十几分钟才失败
type PortProfile struct {
	// Name 服务名，仅用于日志
	Name string
	// ServerFirst 为 true 表示服务端先说话（如 FTP 21），不能等客户端首包
	ServerFirst bool
	// FirstByteTimeout 建议的「首字节超时」，同时也是扫描器探测的拦截窗口
	FirstByteTimeout time.Duration
	// UserTimeout 建议的 TCP_USER_TIMEOUT：对端消失后多久判定连接已死
	UserTimeout time.Duration
	// Known 是否命中内置的常见端口表
	Known bool
}

// 首字节超时：扫描器连上之后通常不会再发应用层数据，正常客户端会在毫秒级发出首包。
// 15 秒对所有人工操作都足够宽松，同时让空连接无法长期占用 goroutine 与 fd。
const (
	defaultFirstByteTimeout = 15 * time.Second
	interactiveUserTimeout  = 2 * time.Minute
	defaultUserTimeout      = 5 * time.Minute
)

var portProfiles = map[int]PortProfile{
	3389: {Name: "rdp", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	22:   {Name: "ssh", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	23:   {Name: "telnet", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	21:   {Name: "ftp", ServerFirst: true, FirstByteTimeout: 0, UserTimeout: defaultUserTimeout, Known: true},
	20:   {Name: "ftp-data", ServerFirst: true, FirstByteTimeout: 0, UserTimeout: defaultUserTimeout, Known: true},
	80:   {Name: "http", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	443:  {Name: "https", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	8080: {Name: "http-alt", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	8443: {Name: "https-alt", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	5900: {Name: "vnc", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	5901: {Name: "vnc", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: interactiveUserTimeout, Known: true},
	1433: {Name: "mssql", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: defaultUserTimeout, Known: true},
	3306: {Name: "mysql", ServerFirst: true, FirstByteTimeout: 0, UserTimeout: defaultUserTimeout, Known: true},
	5432: {Name: "postgres", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: defaultUserTimeout, Known: true},
	6379: {Name: "redis", FirstByteTimeout: defaultFirstByteTimeout, UserTimeout: defaultUserTimeout, Known: true},
}

// ProfileForPort 返回某个端口对应的连接策略。
// 未收录的端口给一套保守默认值：不猜协议、不等首包、只限制失效连接的回收时间。
func ProfileForPort(port int) PortProfile {
	if p, ok := portProfiles[port]; ok {
		return p
	}
	return PortProfile{
		Name:             "",
		ServerFirst:      false,
		FirstByteTimeout: 0,
		UserTimeout:      defaultUserTimeout,
		Known:            false,
	}
}

// KnownPortName 返回常见端口的服务名，未收录时返回空字符串
func KnownPortName(port int) string {
	return portProfiles[port].Name
}
