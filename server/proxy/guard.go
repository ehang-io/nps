package proxy

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

// guardHolder 缓存某个隧道的守卫，配置变化时才重建，重建前的封禁状态随之丢弃
type guardHolder struct {
	key   string
	guard *common.AccessGuard
}

var guardCache sync.Map

// TaskGuard 依据隧道自身配置 + nps.conf 全局默认值构造来源访问守卫。
// 优先级：隧道字段 > nps.conf 的 allow_ip / deny_ip / tunnel_conn_rate。
// 同一隧道配置不变时复用同一个守卫，避免重启任务把封禁名单一起清空。
func TaskGuard(task *file.Tunnel) *common.AccessGuard {
	if task == nil {
		return nil
	}
	allow := strings.TrimSpace(task.IpAcl)
	if allow == "" {
		allow = strings.TrimSpace(beego.AppConfig.DefaultString("allow_ip", ""))
	}
	deny := strings.TrimSpace(task.IpDeny)
	if deny == "" {
		deny = strings.TrimSpace(beego.AppConfig.DefaultString("deny_ip", ""))
	}
	rate := task.ConnRate
	if rate <= 0 {
		rate, _ = beego.AppConfig.Int("tunnel_conn_rate")
	}
	if rate <= 0 {
		//隧道与全局都没配速率时，对 3389/22 这类高危端口套一个宽松的默认值，
		//防止公网爆破脚本无限次尝试导致 Windows/SSH 账号被锁；
		//正常使用（人工重连、RDP 多开）远达不到这个频率
		rate = defaultRiskyPortRate(task.Port)
	}
	window := beego.AppConfig.DefaultInt("tunnel_conn_window", 1)
	if window <= 0 {
		window = 1
	}
	ban := beego.AppConfig.DefaultInt("tunnel_ban_seconds", 600)
	if ban <= 0 {
		ban = 600
	}
	key := allow + "|" + deny + "|" + strconv.Itoa(rate) + "|" + strconv.Itoa(window) + "|" + strconv.Itoa(ban)
	if v, ok := guardCache.Load(task.Id); ok {
		if h, ok := v.(*guardHolder); ok && h.key == key {
			return h.guard
		}
	}
	g := common.NewAccessGuard(allow, deny, rate, time.Duration(window)*time.Second, time.Duration(ban)*time.Second)
	guardCache.Store(task.Id, &guardHolder{key: key, guard: g})
	return g
}

// DropGuardCache 删除指定隧道的守卫缓存
func DropGuardCache(taskId int) {
	guardCache.Delete(taskId)
}

// highRiskPorts 公网上被扫描、爆破最频繁的服务端口。
// 这些端口暴露到公网时默认加一层连接频率保护，避免爆破导致账号被锁定。
var highRiskPorts = map[int]struct{}{
	21:    {}, //FTP
	22:    {}, //SSH
	23:    {}, //Telnet
	135:   {}, //RPC
	139:   {}, //NetBIOS
	445:   {}, //SMB
	1433:  {}, //MSSQL
	1521:  {}, //Oracle
	3306:  {}, //MySQL
	3389:  {}, //RDP
	5432:  {}, //PostgreSQL
	5900:  {}, //VNC
	5985:  {}, //WinRM
	6379:  {}, //Redis
	27017: {}, //MongoDB
}

// defaultRiskyPortRate 返回高危端口的默认连接速率限制，0 表示不启用
func defaultRiskyPortRate(port int) int {
	if _, ok := highRiskPorts[port]; !ok {
		return 0
	}
	if !beego.AppConfig.DefaultBool("risky_port_guard", true) {
		return 0
	}
	rate, _ := beego.AppConfig.Int("risky_port_conn_rate")
	if rate <= 0 {
		rate = 4
	}
	return rate
}

// rejectConn 关闭被拒绝的连接。TCP 设置 linger=0 直接发 RST，
// 让扫描器看到的是“连接被重置/关闭”，而不是一个挂着的、看起来有服务的端口。
func rejectConn(c net.Conn) {
	if c == nil {
		return
	}
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = c.Close()
}

// rejectLogger 汇总被拦截的连接，避免扫描器把日志刷爆（同时也省 CPU 与磁盘）
type rejectLogger struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
	lastIp      string
	lastReason  string
}

var guardRejects = &rejectLogger{}

const rejectLogInterval = 10 * time.Second

func (l *rejectLogger) log(ip, reason string) {
	now := time.Now()
	l.mu.Lock()
	if l.windowStart.IsZero() {
		l.windowStart = now
	}
	l.count++
	l.lastIp = ip
	l.lastReason = reason
	if now.Sub(l.windowStart) < rejectLogInterval {
		l.mu.Unlock()
		return
	}
	count, lastIp, lastReason := l.count, l.lastIp, l.lastReason
	l.count = 0
	l.windowStart = now
	l.mu.Unlock()
	logs.Debug("access guard blocked %d connections in %s, last from %s (%s)", count, rejectLogInterval, lastIp, lastReason)
}

// logReject 记录一次拦截
func logReject(addr net.Addr, reason string) {
	ip := ""
	if addr != nil {
		ip = addr.String()
	}
	guardRejects.log(ip, reason)
}

// guardDescription 便于启动日志里提示当前端口的访问策略
func guardDescription(g *common.AccessGuard, port int) string {
	if g == nil || g.Disabled() {
		return fmt.Sprintf("port %d accepts connections from any address", port)
	}
	desc := fmt.Sprintf("port %d access guard enabled, allow=%q deny=%q", port, g.AllowList(), g.DenyList())
	if rate := g.RateLimit(); rate > 0 {
		desc += fmt.Sprintf(" conn_rate=%d/s", rate)
	}
	return desc
}

// globalGuardCache 针对域名（http/https）代理入口的全局守卫缓存。
// 域名转发没有独立隧道字段，只能按 nps.conf 的 allow_ip/deny_ip 统一限制。
var (
	globalGuardCache sync.Map
)

// GlobalGuard 返回 http/https 代理入口使用的来源守卫
func GlobalGuard() *common.AccessGuard {
	allow := strings.TrimSpace(beego.AppConfig.DefaultString("allow_ip", ""))
	deny := strings.TrimSpace(beego.AppConfig.DefaultString("deny_ip", ""))
	key := allow + "|" + deny
	if v, ok := globalGuardCache.Load(key); ok {
		return v.(*common.AccessGuard)
	}
	g := common.NewAccessGuard(allow, deny, 0, time.Second, time.Minute)
	globalGuardCache.Store(key, g)
	return g
}

// CheckHttpSource 校验 http/https 代理入口的来源地址
func CheckHttpSource(remoteAddr string) bool {
	g := GlobalGuard()
	if g.Disabled() {
		return true
	}
	ok, reason := g.Filter(&stringAddr{remoteAddr})
	if !ok {
		logReject(&stringAddr{remoteAddr}, reason)
	}
	return ok
}

// stringAddr 让 net.Addr 接口能直接承载 "ip:port" 字符串
type stringAddr struct {
	v string
}

func (a *stringAddr) Network() string { return "tcp" }
func (a *stringAddr) String() string  { return a.v }
