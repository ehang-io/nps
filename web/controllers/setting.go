package controllers

// 鉴权与 API 设置页：在 Web 后台直接查看 / 生成 / 保存 auth_key 与 api_key。
//
// 这两个密钥原来只能手改 nps.conf 再重启：
//   - auth_key 配合 timestamp+md5 可以免登录调用后台接口（客户端 GUI 用它拉起页面）
//   - api_key 是 /api/v1 的调用凭据
// 这里把它们做成后台可维护，并提供现成的签名 URL 与 curl 示例，
// 省掉手算 md5、手敲命令的步骤。

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ehang.io/nps/lib/crypt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

// 后台维护的密钥长度范围：太短容易被穷举，太长没必要
const (
	settingKeyMin = 8
	settingKeyMax = 64
	// settingSignTTL 与服务端校验窗口保持一致（BaseController.Prepare 允许 ±20 秒）
	settingSignTTL = 20
)

type SettingController struct {
	BaseController
}

// Index 展示当前密钥与调用示例；POST 保存
func (s *SettingController) Index() {
	s.Data["menu"] = "setting"
	s.SetInfo("setting")
	if s.Data["isAdmin"] != true {
		//子账号不能改全局密钥
		s.error()
		return
	}
	if s.Ctx.Request.Method == http.MethodPost {
		if err := s.saveKeys(); err != nil {
			s.Data["err"] = err.Error()
		} else {
			s.Redirect(s.Data["web_base_url"].(string)+"/setting/index?ok=1", 302)
			return
		}
	}
	s.render()
}

func (s *SettingController) render() {
	authKey := beego.AppConfig.String("auth_key")
	apiKey := beego.AppConfig.String("api_key")
	s.Data["auth_key"] = authKey
	s.Data["api_key"] = apiKey
	s.Data["auth_key_set"] = authKey != ""
	s.Data["api_key_set"] = apiKey != ""
	s.Data["api_allow_ip"] = beego.AppConfig.DefaultString("api_allow_ip", "")
	s.Data["api_read_only"], _ = beego.AppConfig.Bool("api_read_only")
	s.Data["api_rate_limit"] = beego.AppConfig.DefaultInt("api_rate_limit", 0)
	s.Data["saved"] = s.GetString("ok") == "1"
	s.Data["conf_file"] = confFilePath()

	base := s.baseURL()
	s.Data["signed_url"] = signedLoginURL(base, authKey)
	s.Data["curl_example"] = curlExample(base, apiKey)
	s.Data["key_min"] = settingKeyMin
	s.Data["key_max"] = settingKeyMax
	s.display()
}

// saveKeys 校验并保存 auth_key / api_key，写回 nps.conf 并立即生效（不需要重启）
func (s *SettingController) saveKeys() error {
	type item struct{ key, label string }
	items := []item{
		{"auth_key", "auth_key"},
		{"api_key", "api_key"},
	}
	//先整体校验，任何一项非法都不落盘，避免只改了一半
	values := make(map[string]string, len(items))
	for _, it := range items {
		v := strings.TrimSpace(s.GetString(it.key))
		if v != "" {
			if err := validateSettingKey(v); err != nil {
				return fmt.Errorf("%s: %s", it.label, err.Error())
			}
		}
		values[it.key] = v
	}
	path := confFilePath()
	for _, it := range items {
		if err := updateConfValue(path, it.key, values[it.key]); err != nil {
			return fmt.Errorf("write %s to %s failed: %s", it.key, path, err.Error())
		}
	}
	//写盘成功后再更新内存配置，两边都生效
	for _, it := range items {
		if err := beego.AppConfig.Set(it.key, values[it.key]); err != nil {
			return err
		}
	}
	logs.Info("[setting] %s updated auth_key/api_key from %s", s.Ctx.Request.RemoteAddr, path)
	return nil
}

// Generate 返回一个随机密钥，供页面上的「随机生成」按钮使用
func (s *SettingController) Generate() {
	if s.Data["isAdmin"] != true && s.GetSession("auth") != true {
		s.Data["json"] = map[string]interface{}{"status": 0, "msg": "forbidden"}
		s.ServeJSON()
		s.StopRun()
		return
	}
	key, err := randomSettingKey(32)
	if err != nil {
		s.Data["json"] = map[string]interface{}{"status": 0, "msg": err.Error()}
		s.ServeJSON()
		s.StopRun()
		return
	}
	s.Data["json"] = map[string]interface{}{"status": 1, "key": key}
	s.ServeJSON()
	s.StopRun()
}

// baseURL 返回后台对外地址，用于拼示例
func (s *SettingController) baseURL() string {
	base := strings.TrimRight(s.Data["web_base_url"].(string), "/")
	host := s.Ctx.Request.Host
	scheme := "http"
	if s.Ctx.Input.IsSecure() || beego.AppConfig.DefaultString("web_open_ssl", "false") == "true" {
		scheme = "https"
	}
	return scheme + "://" + host + base
}

// validateSettingKey 限制密钥字符集与长度：只允许 URL 与请求头里安全出现的字符
func validateSettingKey(key string) error {
	if len(key) < settingKeyMin || len(key) > settingKeyMax {
		return fmt.Errorf("length must be between %d and %d", settingKeyMin, settingKeyMax)
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("only letters, digits, '-' and '_' are allowed")
		}
	}
	return nil
}

// randomSettingKey 用 crypto/rand 生成密钥
func randomSettingKey(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	key := strings.NewReplacer("+", "", "/", "", "=", "").Replace(base64.RawURLEncoding.EncodeToString(buf))
	if len(key) > n {
		key = key[:n]
	}
	return key, nil
}

// signedLoginURL 生成一次性的免登录地址：md5(auth_key + timestamp) + timestamp
func signedLoginURL(base, authKey string) string {
	if authKey == "" {
		return ""
	}
	ts := time.Now().Unix()
	sign := crypt.Md5(authKey + fmt.Sprint(ts))
	return fmt.Sprintf("%s/login/index?auth_key=%s&timestamp=%d", base, sign, ts)
}

// curlExample 拼一条可直接复制的调用示例
func curlExample(base, apiKey string) string {
	key := apiKey
	if key == "" {
		key = "<api_key>"
	}
	return fmt.Sprintf("curl -s -H \"X-Api-Key: %s\" %s/api/v1/tunnels", key, base)
}
