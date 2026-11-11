package common

import (
	"net"
	"sync"
	"time"
)

// 拒绝原因，便于日志和统计区分
const (
	DenyReasonNone       = ""
	DenyReasonDenyList   = "ip in deny list"
	DenyReasonNotAllowed = "ip not in allow list"
	DenyReasonRateLimit  = "connection rate limit exceeded"
	DenyReasonBanned     = "ip temporarily banned"
)

// guardEntry 记录单个来源 IP 在时间窗内的连接次数与封禁截止时间
type guardEntry struct {
	conns       int
	windowStart time.Time
	bannedUntil time.Time
	lastSeen    time.Time
}

// AccessGuard 给穿透端口做来源校验：
//  1. 黑名单直接拒绝；
//  2. 配置了白名单时只放行白名单；
//  3. 每个来源 IP 在 window 内最多新建 rate 条连接，超限则封禁 banFor 时长。
//
// 命中任何一条都在握手前关闭连接，非法来源既拿不到真实服务的 banner，
// 也不会把请求转发给内网 3389 之类的服务，避免被爆破导致账号锁定。
// 所有状态都放在有界 map 里并定期清理，不会随扫描量无限增长。
type AccessGuard struct {
	allow      *IPList
	deny       *IPList
	rate       int
	window     time.Duration
	banFor     time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]*guardEntry
}

// NewAccessGuard rate<=0 表示不限制新建连接速率；allow/deny 为空表示不限制
func NewAccessGuard(allow, deny string, rate int, window, banFor time.Duration) *AccessGuard {
	g := &AccessGuard{
		allow:      NewIPList(allow),
		deny:       NewIPList(deny),
		rate:       rate,
		window:     window,
		banFor:     banFor,
		maxEntries: 8192,
		now:        time.Now,
		entries:    make(map[string]*guardEntry),
	}
	if g.window <= 0 {
		g.window = time.Second
	}
	if g.banFor <= 0 {
		g.banFor = 10 * time.Minute
	}
	return g
}

// Disabled 表示这个守卫不做任何限制，可以在接入处直接跳过
func (g *AccessGuard) Disabled() bool {
	if g == nil {
		return true
	}
	return g.allow.Empty() && g.deny.Empty() && g.rate <= 0
}

// Filter 只做白/黑名单判断，不参与连接计数，适用于 UDP 这类无连接的转发路径
func (g *AccessGuard) Filter(addr net.Addr) (bool, string) {
	if g == nil {
		return true, DenyReasonNone
	}
	ip := IPFromAddr(addr)
	if ip == nil {
		//无法解析来源的请求一律按非法处理，避免绕过策略
		return false, DenyReasonNotAllowed
	}
	if !g.deny.Empty() && g.deny.Contains(ip) {
		return false, DenyReasonDenyList
	}
	if !g.allow.Empty() && !g.allow.Contains(ip) {
		return false, DenyReasonNotAllowed
	}
	return true, DenyReasonNone
}

// IsDenied 只判断来源是否命中黑名单，不检查白名单与速率。
// 用于「辅助通道的连接方与主通道来源本就不同」的场景（例如 FTP 主动模式的数据
// 连接由内网服务发起）：既拦掉明确封禁的来源，又不会误伤合法传输。
func (g *AccessGuard) IsDenied(addr net.Addr) bool {
	if g == nil || g.DenyList() == "" {
		return false
	}
	ip := IPFromAddr(addr)
	if ip == nil {
		return false
	}
	return g.deny.Contains(ip)
}

// Check 做完整校验（白/黑名单 + 连接速率 + 封禁），返回是否放行以及拒绝原因
func (g *AccessGuard) Check(addr net.Addr) (bool, string) {
	if ok, reason := g.Filter(addr); !ok {
		return false, reason
	}
	if g == nil || g.rate <= 0 {
		return true, DenyReasonNone
	}
	ip := IPFromAddr(addr)
	if ip == nil {
		return false, DenyReasonNotAllowed
	}
	key := ip.String()

	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()

	e, ok := g.entries[key]
	if !ok {
		if len(g.entries) >= g.maxEntries {
			g.sweepLocked(now)
		}
		e = &guardEntry{windowStart: now}
		g.entries[key] = e
	}
	e.lastSeen = now
	if now.Before(e.bannedUntil) {
		return false, DenyReasonBanned
	}
	if now.Sub(e.windowStart) >= g.window {
		e.windowStart = now
		e.conns = 0
	}
	e.conns++
	if e.conns > g.rate {
		e.bannedUntil = now.Add(g.banFor)
		e.conns = 0
		e.windowStart = now
		return false, DenyReasonRateLimit
	}
	return true, DenyReasonNone
}

// sweepLocked 清理过期的窗口与已解封的记录，调用方需持有锁
func (g *AccessGuard) sweepLocked(now time.Time) {
	for k, e := range g.entries {
		if now.Before(e.bannedUntil) {
			continue
		}
		if now.Sub(e.lastSeen) > 2*g.window && now.Sub(e.windowStart) > 2*g.window {
			delete(g.entries, k)
		}
	}
}

// Entries 返回当前记录数，主要用于测试与监控
func (g *AccessGuard) Entries() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.entries)
}

// AllowList 返回白名单原始配置
func (g *AccessGuard) AllowList() string {
	if g == nil {
		return ""
	}
	return g.allow.String()
}

// DenyList 返回黑名单原始配置
func (g *AccessGuard) DenyList() string {
	if g == nil {
		return ""
	}
	return g.deny.String()
}

// RateLimit 返回时间窗内允许的新建连接数，0 表示不限
func (g *AccessGuard) RateLimit() int {
	if g == nil {
		return 0
	}
	return g.rate
}
