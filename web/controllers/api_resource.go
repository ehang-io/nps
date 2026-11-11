package controllers

// /api/v1/clients、/api/v1/hosts、/api/v1/status：客户端、域名隧道与运行状态。
// 客户端与域名是隧道的前置资源，自动化开通（建客户端 -> 建隧道）全靠这几个接口。

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/rate"
	"ehang.io/nps/server"
	"github.com/astaxie/beego"
)

// apiClient 是客户端的对外视图，vkey 默认只给掩码，避免列表接口把连接凭据批量带走
type apiClient struct {
	Id              int    `json:"id"`
	Remark          string `json:"remark"`
	Status          bool   `json:"status"`
	Online          bool   `json:"online"`
	Public          bool   `json:"public"`
	Addr            string `json:"addr,omitempty"`
	Version         string `json:"version,omitempty"`
	RateLimit       int    `json:"rate_limit"`
	MaxConn         int    `json:"max_conn"`
	MaxTunnelNum    int    `json:"max_tunnel_num"`
	NowConn         int32  `json:"now_conn"`
	TunnelCount     int    `json:"tunnel_count"`
	HostCount       int    `json:"host_count"`
	InletFlow       int64  `json:"inlet_flow"`
	ExportFlow      int64  `json:"export_flow"`
	FlowLimit       int64  `json:"flow_limit"`
	Vkey            string `json:"vkey,omitempty"`
	VkeyMasked      string `json:"vkey_masked,omitempty"`
	WebUserName     string `json:"web_username,omitempty"`
	ConfigConnAllow bool   `json:"config_conn_allow"`
}

type apiClientCreate struct {
	Remark          string `json:"remark"`
	Vkey            string `json:"vkey"`
	MaxConn         *int   `json:"max_conn"`
	MaxTunnelNum    *int   `json:"max_tunnel_num"`
	RateLimit       *int   `json:"rate_limit"`
	FlowLimit       *int64 `json:"flow_limit"`
	Status          *bool  `json:"status"`
	ConfigConnAllow *bool  `json:"config_conn_allow"`
	WebUserName     string `json:"web_username"`
	WebPassword     string `json:"web_password"`
	U               string `json:"u"`
	P               string `json:"p"`
}

type apiClientPatch struct {
	Remark          *string `json:"remark"`
	Vkey            *string `json:"vkey"`
	MaxConn         *int    `json:"max_conn"`
	MaxTunnelNum    *int    `json:"max_tunnel_num"`
	RateLimit       *int    `json:"rate_limit"`
	FlowLimit       *int64  `json:"flow_limit"`
	Status          *bool   `json:"status"`
	ConfigConnAllow *bool   `json:"config_conn_allow"`
	WebUserName     *string `json:"web_username"`
	WebPassword     *string `json:"web_password"`
	U               *string `json:"u"`
	P               *string `json:"p"`
}

// apiHost 是域名隧道的对外视图
type apiHost struct {
	Id           int    `json:"id"`
	Host         string `json:"host"`
	Scheme       string `json:"scheme"`
	Location     string `json:"location"`
	Remark       string `json:"remark"`
	Target       string `json:"target,omitempty"`
	LocalProxy   bool   `json:"local_proxy"`
	HeaderChange string `json:"header,omitempty"`
	HostChange   string `json:"host_change,omitempty"`
	CertFilePath string `json:"cert_file,omitempty"`
	KeyFilePath  string `json:"key_file,omitempty"`
	IsClose      bool   `json:"is_close"`
	ClientId     int    `json:"client_id"`
	InletFlow    int64  `json:"inlet_flow"`
	ExportFlow   int64  `json:"export_flow"`
}

type apiHostCreate struct {
	ClientId     int    `json:"client_id"`
	Host         string `json:"host"`
	Scheme       string `json:"scheme"`
	Location     string `json:"location"`
	Remark       string `json:"remark"`
	Target       string `json:"target"`
	LocalProxy   bool   `json:"local_proxy"`
	HeaderChange string `json:"header"`
	HostChange   string `json:"host_change"`
	CertFilePath string `json:"cert_file"`
	KeyFilePath  string `json:"key_file"`
}

func apiMaskVkey(vkey string) string {
	switch {
	case len(vkey) <= 4:
		return strings.Repeat("*", len(vkey))
	case len(vkey) <= 8:
		return vkey[:2] + strings.Repeat("*", len(vkey)-4) + vkey[len(vkey)-2:]
	default:
		return vkey[:3] + strings.Repeat("*", len(vkey)-6) + vkey[len(vkey)-3:]
	}
}

func apiClientView(c *file.Client, revealVkey bool) *apiClient {
	if c == nil {
		return nil
	}
	v := &apiClient{
		Id:              c.Id,
		Remark:          c.Remark,
		Status:          c.Status,
		Addr:            c.Addr,
		Version:         c.Version,
		RateLimit:       c.RateLimit,
		MaxConn:         c.MaxConn,
		MaxTunnelNum:    c.MaxTunnelNum,
		NowConn:         atomic.LoadInt32(&c.NowConn),
		TunnelCount:     c.GetTunnelNum(),
		WebUserName:     c.WebUserName,
		ConfigConnAllow: c.ConfigConnAllow,
		Public:          c.NoDisplay,
		VkeyMasked:      apiMaskVkey(c.VerifyKey),
	}
	if revealVkey {
		v.Vkey = c.VerifyKey
	}
	if c.Flow != nil {
		v.InletFlow = c.Flow.InletFlow
		v.ExportFlow = c.Flow.ExportFlow
		v.FlowLimit = c.Flow.FlowLimit
	}
	if server.Bridge != nil {
		if _, ok := server.Bridge.Client.Load(c.Id); ok {
			v.Online = true
		}
	}
	file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		if h := value.(*file.Host); h.Client != nil && h.Client.Id == c.Id {
			v.HostCount++
		}
		return true
	})
	return v
}

func apiListClients(search string, page, limit int) ([]*file.Client, int) {
	var keys []int
	file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
		if id, ok := key.(int); ok {
			keys = append(keys, id)
		}
		return true
	})
	sort.Ints(keys)
	list := make([]*file.Client, 0, len(keys))
	for _, id := range keys {
		value, ok := file.GetDb().JsonDb.Clients.Load(id)
		if !ok {
			continue
		}
		c := value.(*file.Client)
		if search != "" && !apiClientMatch(c, search) {
			continue
		}
		list = append(list, c)
	}
	total := len(list)
	start := (page - 1) * limit
	if start >= total {
		return []*file.Client{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return list[start:end], total
}

func apiClientMatch(c *file.Client, search string) bool {
	if strconv.Itoa(c.Id) == search {
		return true
	}
	return strings.Contains(c.Remark, search) || strings.Contains(c.VerifyKey, search)
}

// ListClients 列出客户端，include_vkey=true 时返回完整 vkey
func (s *ApiController) ListClients() {
	if !s.apiGuard(false) {
		return
	}
	page, limit, _ := s.apiPage()
	reveal := apiParseBool(s.Ctx.Input.Query("include_vkey"))
	list, total := apiListClients(strings.TrimSpace(s.Ctx.Input.Query("search")), page, limit)
	items := make([]*apiClient, 0, len(list))
	for _, c := range list {
		items = append(items, apiClientView(c, reveal))
	}
	s.apiOK(http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetClient 返回单个客户端，include_vkey=true 时返回完整 vkey
func (s *ApiController) GetClient() {
	if !s.apiGuard(false) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	c, err := file.GetDb().GetClient(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("client %d not found", id))
		return
	}
	s.apiOK(http.StatusOK, apiClientView(c, apiParseBool(s.Ctx.Input.Query("include_vkey"))))
}

// CreateClient 新建客户端，响应里会带上完整 vkey（后续接口默认只给掩码）
func (s *ApiController) CreateClient() {
	if !s.apiGuard(true) {
		return
	}
	var req apiClientCreate
	if err := s.apiBody(&req); err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, err.Error())
		return
	}
	if userName := strings.TrimSpace(req.WebUserName); userName != "" {
		if userName == beego.AppConfig.String("web_username") || !file.GetDb().VerifyUserName(userName, 0) {
			s.apiFail(http.StatusConflict, apiCodeConflict, "web login username is duplicate or reserved")
			return
		}
	}
	c := &file.Client{
		VerifyKey:       strings.TrimSpace(req.Vkey),
		Id:              int(file.GetDb().JsonDb.GetClientId()),
		Status:          true,
		Remark:          strings.TrimSpace(req.Remark),
		Cnf:             &file.Config{U: req.U, P: req.P},
		WebUserName:     strings.TrimSpace(req.WebUserName),
		WebPassword:     req.WebPassword,
		ConfigConnAllow: req.ConfigConnAllow != nil && *req.ConfigConnAllow,
		Flow:            &file.Flow{},
	}
	if req.MaxConn != nil {
		c.MaxConn = *req.MaxConn
	}
	if req.MaxTunnelNum != nil {
		c.MaxTunnelNum = *req.MaxTunnelNum
	}
	if req.RateLimit != nil {
		c.RateLimit = *req.RateLimit
	}
	if req.FlowLimit != nil {
		c.Flow.FlowLimit = *req.FlowLimit
	}
	if req.Status != nil {
		c.Status = *req.Status
	}
	if err := file.GetDb().NewClient(c); err != nil {
		s.apiFail(http.StatusConflict, apiCodeConflict, err.Error())
		return
	}
	s.apiLog("create client", "id=%d remark=%q", c.Id, c.Remark)
	s.apiOK(http.StatusCreated, apiClientView(c, true))
}

// UpdateClient 局部更新客户端
func (s *ApiController) UpdateClient() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	c, err := file.GetDb().GetClient(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("client %d not found", id))
		return
	}
	var req apiClientPatch
	if err := s.apiBody(&req); err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, err.Error())
		return
	}
	if req.Vkey != nil {
		vkey := strings.TrimSpace(*req.Vkey)
		if vkey == "" {
			s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "vkey cannot be empty")
			return
		}
		if !file.GetDb().VerifyVkey(vkey, c.Id) {
			s.apiFail(http.StatusConflict, apiCodeConflict, "vkey is duplicate")
			return
		}
		c.VerifyKey = vkey
	}
	if req.WebUserName != nil {
		userName := strings.TrimSpace(*req.WebUserName)
		if userName != "" {
			if userName == beego.AppConfig.String("web_username") || !file.GetDb().VerifyUserName(userName, c.Id) {
				s.apiFail(http.StatusConflict, apiCodeConflict, "web login username is duplicate or reserved")
				return
			}
		}
		c.WebUserName = userName
	}
	if req.Remark != nil {
		c.Remark = strings.TrimSpace(*req.Remark)
	}
	if req.WebPassword != nil {
		c.WebPassword = *req.WebPassword
	}
	if req.MaxConn != nil {
		c.MaxConn = *req.MaxConn
	}
	if req.MaxTunnelNum != nil {
		c.MaxTunnelNum = *req.MaxTunnelNum
	}
	if req.ConfigConnAllow != nil {
		c.ConfigConnAllow = *req.ConfigConnAllow
	}
	if c.Cnf == nil {
		c.Cnf = new(file.Config)
	}
	if req.U != nil {
		c.Cnf.U = *req.U
	}
	if req.P != nil {
		c.Cnf.P = *req.P
	}
	if req.FlowLimit != nil {
		if c.Flow == nil {
			c.Flow = new(file.Flow)
		}
		c.Flow.FlowLimit = *req.FlowLimit
	}
	if req.RateLimit != nil {
		c.RateLimit = *req.RateLimit
		// 速率对象按新配置重建，旧对象要 Stop，否则 ticker 会一直跑
		if c.Rate != nil {
			c.Rate.Stop()
		}
		if c.RateLimit > 0 {
			c.Rate = rate.NewRate(int64(c.RateLimit * 1024))
		} else {
			c.Rate = rate.NewRate(int64(2 << 23))
		}
		c.Rate.Start()
	}
	if req.Status != nil {
		c.Status = *req.Status
		if !c.Status {
			server.DelClientConnect(c.Id)
		}
	}
	file.GetDb().JsonDb.StoreClientsToJsonFile()
	s.apiLog("update client", "id=%d status=%v", c.Id, c.Status)
	s.apiOK(http.StatusOK, apiClientView(c, false))
}

// DeleteClient 删除客户端。存在隧道/域名时需要 cascade=true，否则返回 409。
func (s *ApiController) DeleteClient() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	c, err := file.GetDb().GetClient(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("client %d not found", id))
		return
	}
	hostCount := 0
	file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		if h := value.(*file.Host); h.Client != nil && h.Client.Id == id {
			hostCount++
		}
		return true
	})
	tunnelCount := c.GetTunnelNum()
	if !apiParseBool(s.Ctx.Input.Query("cascade")) && (tunnelCount > 0 || hostCount > 0) {
		s.apiFail(http.StatusConflict, apiCodeConflict, fmt.Sprintf(
			"client %d still has %d tunnels and %d hosts, pass cascade=true to delete them together", id, tunnelCount, hostCount))
		return
	}
	// 先断开在线连接，再删隧道/域名，最后删客户端，避免运行中的代理持有已删除对象
	server.DelClientConnect(id)
	server.DelTunnelAndHostByClientId(id, false)
	if err := file.GetDb().DelClient(id); err != nil {
		s.apiFail(http.StatusInternalServerError, apiCodeInternal, err.Error())
		return
	}
	s.apiLog("delete client", "id=%d tunnels=%d hosts=%d", id, tunnelCount, hostCount)
	s.apiOK(http.StatusOK, map[string]interface{}{
		"id":      id,
		"deleted": true,
		"tunnels": tunnelCount,
		"hosts":   hostCount,
	})
}

func apiHostView(h *file.Host) *apiHost {
	if h == nil {
		return nil
	}
	v := &apiHost{
		Id:           h.Id,
		Host:         h.Host,
		Scheme:       h.Scheme,
		Location:     h.Location,
		Remark:       h.Remark,
		HeaderChange: h.HeaderChange,
		HostChange:   h.HostChange,
		CertFilePath: h.CertFilePath,
		KeyFilePath:  h.KeyFilePath,
		IsClose:      h.IsClose,
	}
	if h.Target != nil {
		v.Target = h.Target.TargetStr
		v.LocalProxy = h.Target.LocalProxy
	}
	if h.Flow != nil {
		v.InletFlow = h.Flow.InletFlow
		v.ExportFlow = h.Flow.ExportFlow
	}
	if h.Client != nil {
		v.ClientId = h.Client.Id
	}
	return v
}

func apiListHosts(clientId int, search string, page, limit int) ([]*file.Host, int) {
	var keys []int
	file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		if id, ok := key.(int); ok {
			keys = append(keys, id)
		}
		return true
	})
	sort.Ints(keys)
	list := make([]*file.Host, 0, len(keys))
	for _, id := range keys {
		value, ok := file.GetDb().JsonDb.Hosts.Load(id)
		if !ok {
			continue
		}
		h := value.(*file.Host)
		if clientId != 0 && (h.Client == nil || h.Client.Id != clientId) {
			continue
		}
		if search != "" && !(strconv.Itoa(h.Id) == search || strings.Contains(h.Host, search) || strings.Contains(h.Remark, search)) {
			continue
		}
		list = append(list, h)
	}
	total := len(list)
	start := (page - 1) * limit
	if start >= total {
		return []*file.Host{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return list[start:end], total
}

// ListHosts 列出域名隧道
func (s *ApiController) ListHosts() {
	if !s.apiGuard(false) {
		return
	}
	page, limit, _ := s.apiPage()
	list, total := apiListHosts(s.apiQueryInt("client_id", 0), strings.TrimSpace(s.Ctx.Input.Query("search")), page, limit)
	items := make([]*apiHost, 0, len(list))
	for _, h := range list {
		items = append(items, apiHostView(h))
	}
	s.apiOK(http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetHost 返回单个域名隧道
func (s *ApiController) GetHost() {
	if !s.apiGuard(false) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	h, err := file.GetDb().GetHostById(id)
	if err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("host %d not found", id))
		return
	}
	s.apiOK(http.StatusOK, apiHostView(h))
}

// CreateHost 新建域名隧道，创建后立即对 http/https 代理入口生效
func (s *ApiController) CreateHost() {
	if !s.apiGuard(true) {
		return
	}
	var req apiHostCreate
	if err := s.apiBody(&req); err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, err.Error())
		return
	}
	hostName := strings.TrimSpace(req.Host)
	if hostName == "" {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "host is required")
		return
	}
	scheme := strings.ToLower(strings.TrimSpace(req.Scheme))
	switch scheme {
	case "":
		scheme = "all"
	case "all", "http", "https":
	default:
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "scheme must be all, http or https")
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "target is required, format: host:port")
		return
	}
	if (req.CertFilePath == "") != (req.KeyFilePath == "") {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "cert_file and key_file must be set together")
		return
	}
	client, err := file.GetDb().GetClient(req.ClientId)
	if err != nil {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, fmt.Sprintf("client %d not found", req.ClientId))
		return
	}
	h := &file.Host{
		Id:           int(file.GetDb().JsonDb.GetHostId()),
		Host:         hostName,
		Scheme:       scheme,
		Location:     strings.TrimSpace(req.Location),
		Remark:       strings.TrimSpace(req.Remark),
		HeaderChange: req.HeaderChange,
		HostChange:   req.HostChange,
		CertFilePath: strings.TrimSpace(req.CertFilePath),
		KeyFilePath:  strings.TrimSpace(req.KeyFilePath),
		Client:       client,
		Target:       &file.Target{TargetStr: target, LocalProxy: req.LocalProxy},
		Flow:         &file.Flow{},
	}
	if h.Location == "" {
		h.Location = "/"
	}
	if file.GetDb().IsHostExist(h) {
		s.apiFail(http.StatusConflict, apiCodeConflict, "host already exists")
		return
	}
	if err := file.GetDb().NewHost(h); err != nil {
		s.apiFail(http.StatusConflict, apiCodeConflict, err.Error())
		return
	}
	s.apiLog("create host", "id=%d host=%s scheme=%s", h.Id, h.Host, h.Scheme)
	s.apiOK(http.StatusCreated, apiHostView(h))
}

// DeleteHost 删除域名隧道
func (s *ApiController) DeleteHost() {
	if !s.apiGuard(true) {
		return
	}
	id, ok := s.apiId()
	if !ok {
		return
	}
	if _, err := file.GetDb().GetHostById(id); err != nil {
		s.apiFail(http.StatusNotFound, apiCodeNotFound, fmt.Sprintf("host %d not found", id))
		return
	}
	if err := file.GetDb().DelHost(id); err != nil {
		s.apiFail(http.StatusInternalServerError, apiCodeInternal, err.Error())
		return
	}
	s.apiLog("delete host", "id=%d", id)
	s.apiOK(http.StatusOK, map[string]interface{}{"id": id, "deleted": true})
}

// Status 返回仪表盘数据（与后台首页同一份统计）
func (s *ApiController) Status() {
	if !s.apiGuard(false) {
		return
	}
	s.apiOK(http.StatusOK, server.GetDashboardData())
}
