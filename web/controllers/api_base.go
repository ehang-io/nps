package controllers

// /api/v1 的公共基础设施：凭据校验、来源限制、限流、JSON 读写与参数解析。
// 这些接口面向脚本 / CI / 自动化平台，不依赖浏览器会话，因此完全独立于 BaseController，
// 也不会被 Prepare 里的登录跳转影响。

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

const (
	// apiMaxBodyBytes 单次请求体上限，避免一个超大 JSON 把内存吃掉
	apiMaxBodyBytes = 1 << 20
	// apiDefaultRate 每个来源 IP 每分钟允许的请求数
	apiDefaultRate = 300
	// apiMaxPageSize 列表接口单页上限
	apiMaxPageSize = 200
	// apiDefaultPageSize 列表接口默认单页条数
	apiDefaultPageSize = 20
	// apiRateMaxSources 限流窗口的 map 上限，防止伪造来源把内存撑爆
	apiRateMaxSources = 8192
)

// 错误码：调用方按 code 分支处理，message 只用于展示
const (
	apiCodeDisabled        = "api_disabled"
	apiCodeUnauthorized    = "unauthorized"
	apiCodeForbidden       = "forbidden"
	apiCodeNotFound        = "not_found"
	apiCodeInvalidArgument = "invalid_argument"
	apiCodeConflict        = "conflict"
	apiCodeRateLimited     = "rate_limited"
	apiCodeReadOnly        = "read_only"
	apiCodeTooLarge        = "payload_too_large"
	apiCodeInternal        = "internal_error"
)

// apiStartTime 供健康检查返回运行时长
var apiStartTime = time.Now()

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	OK        bool        `json:"ok"`
	Data      interface{} `json:"data,omitempty"`
	Error     *apiError   `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// ApiController 是 /api/v1 下所有管理接口的基类。
// 认证方式：Authorization: Bearer <api_key> 或 X-Api-Key: <api_key>；
// 另外允许直接用后台的 web 账号做 Basic 认证，方便 curl 调试。
type ApiController struct {
	beego.Controller
	requestID string
	remoteIP  net.IP
}

// ApiPublicController 只提供无需认证的探活接口
type ApiPublicController struct {
	beego.Controller
}

// apiGuard 校验 API 开关、来源白名单、限流与凭据。write 为 true 时还要求非只读模式。
// 返回 false 表示响应已经写完，调用方必须立刻 return。
func (s *ApiController) apiGuard(write bool) bool {
	s.requestID = crypt.GetRandomString(8)
	s.Ctx.Output.Header("X-Request-Id", s.requestID)
	s.remoteIP = apiIPFromString(s.Ctx.Request.RemoteAddr)

	if s.Ctx.Request.ContentLength > apiMaxBodyBytes {
		s.apiFail(http.StatusRequestEntityTooLarge, apiCodeTooLarge, "request body must be smaller than 1MB")
		return false
	}

	apiKey := apiConfig("api_key")
	if apiKey == "" {
		s.apiFail(http.StatusServiceUnavailable, apiCodeDisabled,
			"REST API is disabled, set api_key in nps.conf to enable it")
		return false
	}

	allow := common.NewIPList(apiConfig("api_allow_ip"))
	if !allow.Empty() && !allow.Contains(s.remoteIP) {
		logs.Warn("[API] %s rejected by api_allow_ip, path %s", s.Ctx.Request.RemoteAddr, s.Ctx.Request.URL.Path)
		s.apiFail(http.StatusForbidden, apiCodeForbidden, "source ip is not in api_allow_ip")
		return false
	}

	if limit := apiConfigInt("api_rate_limit", apiDefaultRate); !apiRateAllow(s.remoteIP, limit) {
		s.apiFail(http.StatusTooManyRequests, apiCodeRateLimited, "too many requests, please slow down")
		return false
	}

	if !s.apiAuthorized(apiKey) {
		s.Ctx.Output.Header("WWW-Authenticate", `Basic realm="nps api"`)
		logs.Warn("[API] unauthorized request from %s, path %s", s.Ctx.Request.RemoteAddr, s.Ctx.Request.URL.Path)
		s.apiFail(http.StatusUnauthorized, apiCodeUnauthorized, "invalid or missing api credential")
		return false
	}

	if write && apiConfigBool("api_read_only") {
		s.apiFail(http.StatusForbidden, apiCodeReadOnly, "the api is running in read-only mode")
		return false
	}
	return true
}

// apiAuthorized 校验请求凭据。Bearer / X-Api-Key 走 api_key，Basic 走后台 web 账号。
func (s *ApiController) apiAuthorized(apiKey string) bool {
	if v := strings.TrimSpace(s.Ctx.Request.Header.Get("X-Api-Key")); v != "" {
		return apiKeyEqual(v, apiKey)
	}
	header := strings.TrimSpace(s.Ctx.Request.Header.Get("Authorization"))
	if header == "" {
		return false
	}
	scheme, credential, ok := strings.Cut(header, " ")
	if !ok {
		return false
	}
	switch strings.ToLower(scheme) {
	case "bearer", "token":
		return apiKeyEqual(strings.TrimSpace(credential), apiKey)
	case "basic":
		user, pass, ok := s.Ctx.Request.BasicAuth()
		if !ok {
			return false
		}
		webUser := apiConfig("web_username")
		webPass := apiConfig("web_password")
		if webUser == "" || webPass == "" {
			return false
		}
		return apiKeyEqual(user, webUser) && apiKeyEqual(pass, webPass)
	}
	return false
}

// apiKeyEqual 使用常量时间比较，避免通过响应时间猜出密钥
func apiKeyEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *ApiController) apiOK(status int, data interface{}) {
	s.apiWrite(status, apiResponse{OK: true, Data: data})
}

func (s *ApiController) apiFail(status int, code, message string) {
	s.apiWrite(status, apiResponse{OK: false, Error: &apiError{Code: code, Message: message}})
}

func (s *ApiController) apiWrite(status int, payload apiResponse) {
	payload.RequestID = s.requestID
	s.Ctx.Output.SetStatus(status)
	s.Data["json"] = payload
	s.ServeJSON()
	s.StopRun()
}

// apiBody 解析 JSON 请求体。未知字段直接报错，避免字段名拼错却静默生效。
// 这里直接读 Request.Body：beego 默认不开启 CopyRequestBody，Ctx.Input.RequestBody 永远是空的。
func (s *ApiController) apiBody(dst interface{}) error {
	if s.Ctx.Request.ContentLength > apiMaxBodyBytes {
		return errors.New("request body must be smaller than 1MB")
	}
	if s.Ctx.Request.Body == nil {
		return errors.New("request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(s.Ctx.Request.Body, apiMaxBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if len(body) == 0 {
		return errors.New("request body is required")
	}
	if len(body) > apiMaxBodyBytes {
		return errors.New("request body must be smaller than 1MB")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json body: %w", err)
	}
	return nil
}

// apiId 解析路径里的 :id
func (s *ApiController) apiId() (int, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(s.Ctx.Input.Param(":id")))
	if err != nil || id <= 0 {
		s.apiFail(http.StatusBadRequest, apiCodeInvalidArgument, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

// apiPage 解析 page/limit，返回页码、单页条数与偏移量
func (s *ApiController) apiPage() (page, limit, offset int) {
	limit = s.apiQueryInt("limit", apiDefaultPageSize)
	if limit <= 0 {
		limit = apiDefaultPageSize
	}
	if limit > apiMaxPageSize {
		limit = apiMaxPageSize
	}
	page = s.apiQueryInt("page", 1)
	if page <= 0 {
		page = 1
	}
	return page, limit, (page - 1) * limit
}

func (s *ApiController) apiQueryInt(key string, def int) int {
	raw := strings.TrimSpace(s.Ctx.Input.Query(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

// apiParseBool 解析查询参数里的布尔值，兼容 true/1/yes/on
func apiParseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// apiLog 记录写操作，便于事后追溯是谁改了什么
func (s *ApiController) apiLog(action, format string, args ...interface{}) {
	logs.Info("[API] %s %s %s from %s: %s", s.Ctx.Request.Method, s.Ctx.Request.URL.Path,
		action, s.Ctx.Request.RemoteAddr, fmt.Sprintf(format, args...))
}

func apiConfig(key string) string {
	return strings.TrimSpace(beego.AppConfig.String(key))
}

func apiConfigInt(key string, def int) int {
	if v, err := beego.AppConfig.Int(key); err == nil {
		return v
	}
	return def
}

func apiConfigBool(key string) bool {
	if v, err := beego.AppConfig.Bool(key); err == nil {
		return v
	}
	return false
}

// apiIPFromString 从 "ip:port" 里取出 IP
func apiIPFromString(addr string) net.IP {
	addr = strings.TrimSpace(addr)
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	return net.ParseIP(strings.Trim(addr, "[]"))
}

type apiRateWindow struct {
	count int
	reset time.Time
}

var (
	apiRateMu      sync.Mutex
	apiRateWindows = make(map[string]*apiRateWindow)
	apiRateClean   time.Time
)

// apiRateAllow 固定窗口限流，按来源 IP 每分钟计数。limit <= 0 表示不限制。
func apiRateAllow(ip net.IP, limit int) bool {
	if limit <= 0 || ip == nil {
		return true
	}
	key := ip.String()
	now := time.Now()
	apiRateMu.Lock()
	defer apiRateMu.Unlock()
	if now.Sub(apiRateClean) >= time.Minute {
		for k, w := range apiRateWindows {
			if now.After(w.reset) {
				delete(apiRateWindows, k)
			}
		}
		apiRateClean = now
	}
	w, ok := apiRateWindows[key]
	if !ok || now.After(w.reset) {
		if !ok && len(apiRateWindows) >= apiRateMaxSources {
			return false
		}
		apiRateWindows[key] = &apiRateWindow{count: 1, reset: now.Add(time.Minute)}
		return true
	}
	w.count++
	return w.count <= limit
}

// Health 是探活接口，不需要凭据，也不返回任何敏感信息
func (s *ApiPublicController) Health() {
	s.Data["json"] = apiResponse{
		OK: true,
		Data: map[string]interface{}{
			"status":         "ok",
			"uptime_seconds": int(time.Since(apiStartTime).Seconds()),
			"api_enabled":    apiConfig("api_key") != "",
			"time":           time.Now().Unix(),
		},
	}
	s.ServeJSON()
}
