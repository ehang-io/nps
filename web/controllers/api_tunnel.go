package controllers

// /api/v1/tunnels：穿透项目（隧道）的增删改查与启停。
// 设计要点：
//  1. 请求先用 apiTunnelSpec 拼出完整的期望配置并校验，通过后才写回 *file.Tunnel，
//     这样一次失败的 PATCH 不会把线上隧道改成半截状态；
//  2. 密码类字段（secret/p2p 密码、Basic 认证密码）只写不读，任何查询接口都不返回；
//  3. 端口变更、模式变更会重建监听，认证 / 白名单变更不需要断开已有连接。

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server"
	"ehang.io/nps/server/tool"
	"github.com/astaxie/beego/logs"
)

// apiTunnelModes 允许通过 API 创建的隧道类型（tcpTrans 等内部模式不对外开放）
var apiTunnelModes = map[string]bool{
	"tcp":       true,
	"udp":       true,
	"socks5":    true,
	"httpProxy": true,
	"ftp":       true,
	"secret":    true,
	"p2p":       true,
	"file":      true,
}

// apiTunnel 是对外暴露的隧道视图
type apiTunnel struct {
	Id           int      `json:"id"`
	Mode         string   `json:"mode"`
	Remark       string   `json:"remark"`
	Port         int      `json:"port"`
	PortService  string   `json:"port_service,omitempty"`
	ServerIp     string   `json:"server_ip,omitempty"`
	Target       string   `json:"target,omitempty"`
	LocalProxy   bool     `json:"local_proxy"`
	Status       bool     `json:"status"`
	Running      bool     `json:"running"`
	ClientId     int      `json:"client_id"`
	ClientRemark string   `json:"client_remark,omitempty"`
	IpAcl        string   `json:"ip_acl,omitempty"`
	IpDeny       string   `json:"ip_deny,omitempty"`
	ConnRate     int      `json:"conn_rate"`
	AuthEnabled  bool     `json:"auth_enabled"`
	AuthUser     string   `json:"auth_user,omitempty"`
	TlsEnable    bool     `json:"tls_enable"`
	TlsCertFile  string   `json:"tls_cert_file,omitempty"`
	TlsKeyFile   string   `json:"tls_key_file,omitempty"`
	LocalPath    string   `json:"local_path,omitempty"`
	StripPre     string   `json:"strip_pre,omitempty"`
	InletFlow    int64    `json:"inlet_flow"`
	ExportFlow   int64    `json:"export_flow"`
	Warnings     []string `json:"warnings,omitempty"`
}

// apiTunnelSpec 是隧道在 API 里的可写字段集合
type apiTunnelSpec struct {
	ClientId    int
	Mode        string
	Port        int
	ServerIp    string
	Target      string
	LocalProxy  bool
	Remark      string
	IpAcl       string
	IpDeny      string
	ConnRate    int
	AuthUser    string
	AuthPass    string
	TlsEnable   bool
	TlsCertFile string
	TlsKeyFile  string
	LocalPath   string
	StripPre    string
	Password    string
	Status      bool
}

// apiTunnelCreate 是创建请求体，除 client_id 外都有默认值
type apiTunnelCreate struct {
	ClientId    int    `json:"client_id"`
	Mode        string `json:"mode"`
	Port        int    `json:"port"`
	ServerIp    string `json:"server_ip"`
	Target      string `json:"target"`
	LocalProxy  bool   `json:"local_proxy"`
	Remark      string `json:"remark"`
	IpAcl       string `json:"ip_acl"`
	IpDeny      string `json:"ip_deny"`
	ConnRate    int    `json:"conn_rate"`
	AuthUser    string `json:"auth_user"`
	AuthPass    string `json:"auth_pass"`
	TlsEnable   bool   `json:"tls_enable"`
	TlsCertFile string `json:"tls_cert_file"`
	TlsKeyFile  string `json:"tls_key_file"`
	LocalPath   string `json:"local_path"`
	StripPre    string `json:"strip_pre"`
	Password    string `json:"password"`
	Status      *bool  `json:"status"`
}

// apiTunnelPatch 是更新请求体，全部字段可选，未出现的字段保持原值
type apiTunnelPatch struct {
	ClientId    *int    `json:"client_id"`
	Mode        *string `json:"mode"`
	Port        *int    `json:"port"`
	ServerIp    *string `json:"server_ip"`
	Target      *string `json:"target"`
	LocalProxy  *bool   `json:"local_proxy"`
	Remark      *string `json:"remark"`
	IpAcl       *string `json:"ip_acl"`
	IpDeny      *string `json:"ip_deny"`
	ConnRate    *int    `json:"conn_rate"`
	AuthUser    *string `json:"auth_user"`
	AuthPass    *string `json:"auth_pass"`
	TlsEnable   *bool   `json:"tls_enable"`
	TlsCertFile *string `json:"tls_cert_file"`
	TlsKeyFile  *string `json:"tls_key_file"`
	LocalPath   *string `json:"local_path"`
	StripPre    *string `json:"strip_pre"`
	Password    *string `json:"password"`
	Status      *bool   `json:"status"`
}

func apiTunnelView(t *file.Tunnel, withWarnings bool) *apiTunnel {
	if t == nil {
		return nil
	}
	v := &apiTunnel{
		Id:          t.Id,
		Mode:        t.Mode,
		Remark:      t.Remark,
		Port:        t.Port,
		ServerIp:    t.ServerIp,
		Status:      t.Status,
		IpAcl:       t.IpAcl,
		IpDeny:      t.IpDeny,
		ConnRate:    t.ConnRate,
		AuthEnabled: t.AuthEnabled(),
		AuthUser:    t.AuthUser,
		TlsEnable:   t.TlsEnable,
		TlsCertFile: t.TlsCertFile,
		TlsKeyFile:  t.TlsKeyFile,
		LocalPath:   t.LocalPath,
		StripPre:    t.StripPre,
		PortService: common.KnownPortName(t.Port),
	}
	if t.Client != nil {
		v.ClientId = t.Client.Id
		v.ClientRemark = t.Client.Remark
	}
	if t.Target != nil {
		v.Target = t.Target.TargetStr
		v.LocalProxy = t.Target.LocalProxy
	}
	if t.Flow != nil {
		v.InletFlow = t.Flow.InletFlow
		v.ExportFlow = t.Flow.ExportFlow
	}
	if apiTunnelRunning(t) {
		v.Running = true
	}
	if withWarnings {
		v.Warnings = apiTunnelWarnings(v)
	}
	return v
}

// apiTunnelWarnings 给出不阻断、但值得提醒的配置问题
func apiTunnelWarnings(v *apiTunnel) []string {
	var warnings []string
	if v.Mode != "tcp" {
		return nil
	}
	if v.PortService != "" && !v.AuthEnabled && v.IpAcl == "" {
		warnings = append(warnings, fmt.Sprintf(
			"port %d (%s) is public without auth or ip_acl, scanners and brute force scripts can reach the target service",
			v.Port, v.PortService))
	}
	if v.AuthEnabled && !v.TlsEnable {
		warnings = append(warnings, "basic auth runs over plain tcp, credentials are not encrypted, consider tls_enable")
	}
	return warnings
}

func apiSpecFromTunnel(t *file.Tunnel) *apiTunnelSpec {
	spec := &apiTunnelSpec{
		Mode:        t.Mode,
		Port:        t.Port,
		ServerIp:    t.ServerIp,
		Remark:      t.Remark,
		IpAcl:       t.IpAcl,
		IpDeny:      t.IpDeny,
		ConnRate:    t.ConnRate,
		AuthUser:    t.AuthUser,
		AuthPass:    t.AuthPass,
		TlsEnable:   t.TlsEnable,
		TlsCertFile: t.TlsCertFile,
		TlsKeyFile:  t.TlsKeyFile,
		LocalPath:   t.LocalPath,
		StripPre:    t.StripPre,
		Password:    t.Password,
		Status:      t.Status,
	}
	if t.Client != nil {
		spec.ClientId = t.Client.Id
	}
	if t.Target != nil {
		spec.Target = t.Target.TargetStr
		spec.LocalProxy = t.Target.LocalProxy
	}
	return spec
}

func apiSpecFromCreate(req *apiTunnelCreate) *apiTunnelSpec {
	spec := &apiTunnelSpec{
		ClientId:    req.ClientId,
		Mode:        strings.TrimSpace(req.Mode),
		Port:        req.Port,
		ServerIp:    strings.TrimSpace(req.ServerIp),
		Target:      strings.TrimSpace(req.Target),
		LocalProxy:  req.LocalProxy,
		Remark:      strings.TrimSpace(req.Remark),
		IpAcl:       strings.TrimSpace(req.IpAcl),
		IpDeny:      strings.TrimSpace(req.IpDeny),
		ConnRate:    req.ConnRate,
		AuthUser:    req.AuthUser,
		AuthPass:    req.AuthPass,
		TlsEnable:   req.TlsEnable,
		TlsCertFile: strings.TrimSpace(req.TlsCertFile),
		TlsKeyFile:  strings.TrimSpace(req.TlsKeyFile),
		LocalPath:   strings.TrimSpace(req.LocalPath),
		StripPre:    req.StripPre,
		Password:    req.Password,
		Status:      true,
	}
	if spec.Mode == "" {
		spec.Mode = "tcp"
	}
	if req.Status != nil {
		spec.Status = *req.Status
	}
	return spec
}

func (s *apiTunnelSpec) applyPatch(p *apiTunnelPatch) {
	if p.ClientId != nil {
		s.ClientId = *p.ClientId
	}
	if p.Mode != nil {
		s.Mode = strings.TrimSpace(*p.Mode)
	}
	if p.Port != nil {
		s.Port = *p.Port
	}
	if p.ServerIp != nil {
		s.ServerIp = strings.TrimSpace(*p.ServerIp)
	}
	if p.Target != nil {
		s.Target = strings.TrimSpace(*p.Target)
	}
	if p.LocalProxy != nil {
		s.LocalProxy = *p.LocalProxy
	}
	if p.Remark != nil {
		s.Remark = strings.TrimSpace(*p.Remark)
	}
	if p.IpAcl != nil {
		s.IpAcl = strings.TrimSpace(*p.IpAcl)
	}
	if p.IpDeny != nil {
		s.IpDeny = strings.TrimSpace(*p.IpDeny)
	}
	if p.ConnRate != nil {
		s.ConnRate = *p.ConnRate
	}
	if p.AuthUser != nil {
		s.AuthUser = *p.AuthUser
	}
	if p.AuthPass != nil {
		s.AuthPass = *p.AuthPass
	}
	if p.TlsEnable != nil {
		s.TlsEnable = *p.TlsEnable
	}
	if p.TlsCertFile != nil {
		s.TlsCertFile = strings.TrimSpace(*p.TlsCertFile)
	}
	if p.TlsKeyFile != nil {
		s.TlsKeyFile = strings.TrimSpace(*p.TlsKeyFile)
	}
	if p.LocalPath != nil {
		s.LocalPath = strings.TrimSpace(*p.LocalPath)
	}
	if p.StripPre != nil {
		s.StripPre = *p.StripPre
	}
	if p.Password != nil {
		s.Password = *p.Password
	}
	if p.Status != nil {
		s.Status = *p.Status
	}
}

// validate 校验期望配置。prev 为 nil 表示新建；端口未变化时不重复探测，
// 否则会把自己正在监听的端口判成“被占用”。
func (s *apiTunnelSpec) validate(prev *file.Tunnel) error {
	if !apiTunnelModes[s.Mode] {
		return errors.New("mode must be one of tcp, udp, socks5, httpProxy, ftp, secret, p2p, file")
	}
	if s.ClientId <= 0 {
		return errors.New("client_id is required")
	}
	if s.Mode == "secret" || s.Mode == "p2p" {
		s.Password = strings.TrimSpace(s.Password)
		if s.Password == "" {
			return errors.New("password is required for secret and p2p tunnels")
		}
		s.Port = 0
	} else {
		if s.Port <= 0 || s.Port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
		if s.Mode != "file" && s.Mode != "socks5" && s.Target == "" {
			return errors.New("target is required, format: host:port")
		}
	}
	if s.AuthUser != "" || s.AuthPass != "" {
		if s.Mode != "tcp" {
			return errors.New("basic auth is only supported by tcp tunnels")
		}
		if s.AuthUser == "" || s.AuthPass == "" {
			return errors.New("auth_user and auth_pass must be set together")
		}
	}
	if s.TlsEnable {
		if s.Mode != "tcp" {
			return errors.New("tls is only supported by tcp tunnels")
		}
		if (s.TlsCertFile == "") != (s.TlsKeyFile == "") {
			return errors.New("tls_cert_file and tls_key_file must be set together")
		}
	}
	if s.ConnRate < 0 {
		return errors.New("conn_rate must be greater than or equal to 0")
	}
	if err := apiCheckIPList("ip_acl", s.IpAcl); err != nil {
		return err
	}
	if err := apiCheckIPList("ip_deny", s.IpDeny); err != nil {
		return err
	}
	if s.Mode == "secret" || s.Mode == "p2p" {
		return nil
	}
	family := apiTunnelFamily(s.Mode)
	portChanged := prev == nil || prev.Port != s.Port || apiTunnelFamily(prev.Mode) != family
	if !portChanged {
		return nil
	}
	exclude := 0
	if prev != nil {
		exclude = prev.Id
	}
	if owner, used := apiTunnelPortOwner(s.Port, s.Mode, exclude); used {
		return newAPIConflict("port %d is already used by tunnel %d", s.Port, owner)
	}
	if !tool.TestServerPort(s.Port, s.Mode) {
		return newAPIConflict("port %d cannot be opened, it may be occupied by another process or not allowed by allow_ports", s.Port)
	}
	return nil
}

func (s *apiTunnelSpec) apply(t *file.Tunnel, client *file.Client) {
	t.Port = s.Port
	t.ServerIp = s.ServerIp
	t.IpAcl = s.IpAcl
	t.IpDeny = s.IpDeny
	t.ConnRate = s.ConnRate
	t.Mode = s.Mode
	t.Remark = s.Remark
	t.Password = s.Password
	t.AuthUser = s.AuthUser
	t.AuthPass = s.AuthPass
	t.TlsEnable = s.TlsEnable
	t.TlsCertFile = s.TlsCertFile
	t.TlsKeyFile = s.TlsKeyFile
	t.LocalPath = s.LocalPath
	t.StripPre = s.StripPre
	t.Client = client
	t.Target = &file.Target{TargetStr: s.Target, LocalProxy: s.LocalProxy}
	t.Status = s.Status
}

// apiTunnelFamily 把模式归成占用端口的类别：tcp 类与 udp 类可以复用同一个端口号
func apiTunnelFamily(mode string) string {
	switch mode {
	case "secret", "p2p":
		return ""
	case "udp":
		return "udp"
	}
	return "tcp"
}

func apiTunnelPortOwner(port int, mode string, excludeId int) (int, bool) {
	family := apiTunnelFamily(mode)
	if family == "" || port <= 0 {
		return 0, false
	}
	var owner int
	file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		t := value.(*file.Tunnel)
		if t.Id == excludeId || t.Port != port {
			return true
		}
		if apiTunnelFamily(t.Mode) != family {
			return true
		}
		owner = t.Id
		return false
	})
	return owner, owner != 0
}

func apiCheckIPList(field, raw string) error {
	if raw == "" {
		return nil
	}
	if common.NewIPList(raw).Empty() {
		return fmt.Errorf("%s %q is not a valid ip/cidr list", field, raw)
	}
	return nil
}

// apiSpecNeedsRestart 判断哪些改动必须重建监听。
// 认证、来源白名单、限速在每次连接时读取，改了不需要断开已有连接。
func apiSpecNeedsRestart(spec *apiTunnelSpec, prev *file.Tunnel) bool {
	if spec.Mode != prev.Mode || spec.Port != prev.Port || spec.ServerIp != prev.ServerIp {
		return true
	}
	if spec.TlsEnable != prev.TlsEnable || spec.TlsCertFile != prev.TlsCertFile || spec.TlsKeyFile != prev.TlsKeyFile {
		return true
	}
	if spec.LocalPath != prev.LocalPath || spec.StripPre != prev.StripPre {
		return true
	}
	return false
}

// apiConflictError 表示资源冲突（端口已被其它隧道占用等），handler 会映射成 409
type apiConflictError struct {
	msg string
}

func (e *apiConflictError) Error() string { return e.msg }

func newAPIConflict(format string, args ...interface{}) error {
	return &apiConflictError{msg: fmt.Sprintf(format, args...)}
}

// apiErrorStatus 把校验错误映射成 HTTP 状态码与错误码
func apiErrorStatus(err error) (int, string) {
	var conflict *apiConflictError
	if errors.As(err, &conflict) {
		return http.StatusConflict, apiCodeConflict
	}
	return http.StatusBadRequest, apiCodeInvalidArgument
}

// apiTunnelRunning 判断隧道是否已经在运行。
// RunList 里存 nil 有两种含义：secret/p2p 任务只登记不监听；
// 历史数据里还可能有被误塞进来的客户端 id，所以 mode 不是 secret/p2p 时按未运行处理。
func apiTunnelRunning(t *file.Tunnel) bool {
	if t == nil {
		return false
	}
	v, ok := server.RunList.Load(t.Id)
	if !ok {
		return false
	}
	if v == nil {
		return t.Mode == "secret" || t.Mode == "p2p"
	}
	return true
}

// apiStartTunnel 启动任务并持久化状态。server.StartTask 会忽略 AddTask 的错误，
// API 里需要把启动失败如实返回给调用方。
func apiStartTunnel(t *file.Tunnel) error {
	if apiTunnelRunning(t) {
		logs.Info("[API] tunnel %d is already running, skip start", t.Id)
		return nil
	}
	if err := server.AddTask(t); err != nil {
		logs.Warn("[API] tunnel %d start failed: %s", t.Id, err.Error())
		return err
	}
	t.Status = true
	if err := file.GetDb().UpdateTask(t); err != nil {
		return err
	}
	logs.Info("[API] tunnel %d start requested, mode=%s port=%d", t.Id, t.Mode, t.Port)
	return nil
}

func apiListTunnels(mode string, clientId int, search, state string, page, limit int) ([]*file.Tunnel, int) {
	var keys []int
	file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		if id, ok := key.(int); ok {
			keys = append(keys, id)
		}
		return true
	})
	sort.Ints(keys)
	list := make([]*file.Tunnel, 0, len(keys))
	for _, id := range keys {
		value, ok := file.GetDb().JsonDb.Tasks.Load(id)
		if !ok {
			continue
		}
		t := value.(*file.Tunnel)
		if mode != "" && t.Mode != mode {
			continue
		}
		if clientId != 0 && (t.Client == nil || t.Client.Id != clientId) {
			continue
		}
		if search != "" && !apiTunnelMatch(t, search) {
			continue
		}
		running := apiTunnelRunning(t)
		switch state {
		case "running":
			if !running {
				continue
			}
		case "stopped":
			if running {
				continue
			}
		}
		list = append(list, t)
	}
	total := len(list)
	start := (page - 1) * limit
	if start >= total {
		return []*file.Tunnel{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return list[start:end], total
}

func apiTunnelMatch(t *file.Tunnel, search string) bool {
	if strconv.Itoa(t.Id) == search || strconv.Itoa(t.Port) == search {
		return true
	}
	return strings.Contains(t.Remark, search)
}

// ListTunnels 列出隧道，支持 mode / client_id / search / state / 分页
func (s *ApiController) ListTunnels() {
	if !s.apiGuard(false) {
		return
	}
	page, limit, _ := s.apiPage()
	mode := strings.TrimSpace(s.Ctx.Input.Query("mode"))
	state := strings.ToLower(strings.TrimSpace(s.Ctx.Input.Query("state")))
	if mode != "" && !apiTunnelModes[mode] {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "mode must be one of tcp, udp, socks5, httpProxy, ftp, secret, p2p, file")
		return
	}
	switch state {
	case "", "all", "running", "stopped":
	default:
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "state must be one of all, running, stopped")
		return
	}
	if state == "all" {
		state = ""
	}
	list, total := apiListTunnels(mode, s.apiQueryInt("client_id", 0), strings.TrimSpace(s.Ctx.Input.Query("search")), state, page, limit)
	items := make([]*apiTunnel, 0, len(list))
	for _, t := range list {
		items = append(items, apiTunnelView(t, false))
	}
	s.apiOK(http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetTunnel 返回单个隧道详情，附带配置提醒
func (s *ApiController) GetTunnel() {
	if !s.apiGuard(false) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	t, err := file.GetDb().GetTask(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("tunnel %d not found", id))
		return
	}
	s.apiOK(http.StatusOK, apiTunnelView(t, true))
}

// CreateTunnel 新建并启动隧道
func (s *ApiController) CreateTunnel() {
	if !s.apiGuard(true) {
		return
	}
	var req apiTunnelCreate
	if err := s.apiBody(&req); err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, err.Error())
		return
	}
	spec := apiSpecFromCreate(&req)
	client, err := file.GetDb().GetClient(spec.ClientId)
	if err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, fmt.Sprintf("client %d not found", spec.ClientId))
		return
	}
	if client.MaxTunnelNum != 0 && client.GetTunnelNum() >= client.MaxTunnelNum {
		s.apiFail(http.StatusConflict, apiCodeConflict, "the number of tunnels exceeds the limit of this client")
		return
	}
	if err := spec.validate(nil); err != nil {
		status, code := apiErrorStatus(err)
		s.apiFail(status, code, err.Error())
		return
	}
	t := &file.Tunnel{
		Id:   int(file.GetDb().JsonDb.GetTaskId()),
		Flow: &file.Flow{},
	}
	spec.apply(t, client)
	if err := file.GetDb().NewTask(t); err != nil {
		s.apiFail(http.StatusConflict, apiCodeConflict, err.Error())
		return
	}
	if t.Status {
		if err := apiStartTunnel(t); err != nil {
			// 起不来就回滚，避免留下一条永远启动不了的配置
			_ = file.GetDb().DelTask(t.Id)
			s.apiFail(http.StatusConflict, apiCodeConflict, "the tunnel cannot be started: "+err.Error())
			return
		}
	}
	s.apiLog("create tunnel", "id=%d mode=%s port=%d client=%d", t.Id, t.Mode, t.Port, client.Id)
	s.apiOK(http.StatusCreated, apiTunnelView(t, true))
}

// UpdateTunnel 局部更新隧道，只改请求体里出现的字段
func (s *ApiController) UpdateTunnel() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	t, err := file.GetDb().GetTask(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("tunnel %d not found", id))
		return
	}
	var req apiTunnelPatch
	if err := s.apiBody(&req); err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, err.Error())
		return
	}
	spec := apiSpecFromTunnel(t)
	spec.applyPatch(&req)
	client, err := file.GetDb().GetClient(spec.ClientId)
	if err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, fmt.Sprintf("client %d not found", spec.ClientId))
		return
	}
	if t.Client != nil && spec.ClientId != t.Client.Id && client.MaxTunnelNum != 0 && client.GetTunnelNum() >= client.MaxTunnelNum {
		s.apiFail(http.StatusConflict, apiCodeConflict, "the number of tunnels exceeds the limit of this client")
		return
	}
	if err := spec.validate(t); err != nil {
		status, code := apiErrorStatus(err)
		s.apiFail(status, code, err.Error())
		return
	}
	wasRunning := apiTunnelRunning(t)
	needRestart := wasRunning && apiSpecNeedsRestart(spec, t)
	spec.apply(t, client)
	if err := file.GetDb().UpdateTask(t); err != nil {
		s.apiFail(http.StatusInternalServerError, apiCodeInternal, err.Error())
		return
	}
	if needRestart {
		_ = server.StopServer(id)
	}
	if spec.Status {
		if !apiTunnelRunning(t) {
			if err := apiStartTunnel(t); err != nil {
				s.apiFail(http.StatusConflict, apiCodeConflict, "the tunnel cannot be started: "+err.Error())
				return
			}
		}
	} else if apiTunnelRunning(t) {
		_ = server.StopServer(id)
	}
	s.apiLog("update tunnel", "id=%d mode=%s port=%d running=%v", t.Id, t.Mode, t.Port, spec.Status)
	s.apiOK(http.StatusOK, apiTunnelView(t, true))
}

// DeleteTunnel 停止并删除隧道
func (s *ApiController) DeleteTunnel() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	if _, err := file.GetDb().GetTask(id); err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("tunnel %d not found", id))
		return
	}
	if err := server.DelTask(id); err != nil {
		s.apiFail(http.StatusInternalServerError, apiCodeInternal, err.Error())
		return
	}
	s.apiLog("delete tunnel", "id=%d", id)
	s.apiOK(http.StatusOK, map[string]interface{}{"id": id, "deleted": true})
}

// StartTunnel 启动已存在的隧道
func (s *ApiController) StartTunnel() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	t, err := file.GetDb().GetTask(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("tunnel %d not found", id))
		return
	}
	if apiTunnelRunning(t) {
		s.apiFail(http.StatusConflict, apiCodeConflict, "tunnel is already running")
		return
	}
	if err := apiStartTunnel(t); err != nil {
		s.apiFail(http.StatusConflict, apiCodeConflict, "the tunnel cannot be started: "+err.Error())
		return
	}
	s.apiLog("start tunnel", "id=%d", id)
	s.apiOK(http.StatusOK, apiTunnelView(t, true))
}

// StopTunnel 停止运行中的隧道，配置保留
func (s *ApiController) StopTunnel() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	t, err := file.GetDb().GetTask(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("tunnel %d not found", id))
		return
	}
	if !apiTunnelRunning(t) {
		s.apiFail(http.StatusConflict, apiCodeConflict, "tunnel is not running")
		return
	}
	if err := server.StopServer(id); err != nil {
		s.apiFail(http.StatusInternalServerError, apiCodeInternal, err.Error())
		return
	}
	s.apiLog("stop tunnel", "id=%d", id)
	s.apiOK(http.StatusOK, apiTunnelView(t, true))
}
