package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/client"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/daemon"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/astaxie/beego/logs"
)

func main() {
	daemon.InitDaemon("npc", common.GetRunPath(), common.GetTmpPath())
	logs.SetLogger("store")
	application := app.New()
	window := application.NewWindow("Npc " + version.VERSION)
	window.SetContent(WidgetScreen())
	window.Resize(fyne.NewSize(940, 640))

	window.ShowAndRun()
}

type guiConfig struct {
	server   string
	vkey     string
	connType string
	verify   bool
	proxyUrl string
	user     string
	pass     string
}

type clientGroup struct {
	mu      sync.Mutex
	running bool
	clients []*client.TRPClient
}

var group = &clientGroup{}

func WidgetScreen() fyne.CanvasObject {
	serverEntry := widget.NewEntry()
	serverEntry.SetPlaceHolder("Server:Port，多个服务端用逗号分隔")

	vKeyEntry := widget.NewPasswordEntry()
	vKeyEntry.SetPlaceHolder("Vkey")

	connSelect := widget.NewSelect([]string{"tcp", "kcp", "tls"}, nil)
	connSelect.SetSelected("tcp")

	verifyCheck := widget.NewCheck("校验服务端 TLS 证书（自签证书时保持关闭）", nil)

	proxyEntry := widget.NewEntry()
	proxyEntry.SetPlaceHolder("可选：socks5://user:pass@127.0.0.1:1080 或 http 代理")

	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("可选：Basic 认证用户名（public_vkey 模式）")

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("可选：Basic 认证密码（public_vkey 模式）")

	statusLabel := widget.NewLabel("未连接")
	logEntry := widget.NewMultiLineEntry()
	logEntry.Disable()
	logEntry.Wrapping = fyne.TextWrapWord
	logScroll := container.NewScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(920, 260))

	go func() {
		for {
			time.Sleep(time.Second)
			logEntry.SetText(common.GetLogMsg())
		}
	}()

	config := loadConfig()
	if config.server != "" {
		serverEntry.SetText(config.server)
		vKeyEntry.SetText(config.vkey)
		connSelect.SetSelected(config.connType)
		verifyCheck.SetChecked(config.verify)
		proxyEntry.SetText(config.proxyUrl)
		userEntry.SetText(config.user)
		passEntry.SetText(config.pass)
	}

	startBtn := widget.NewButton("启动", nil)
	startBtn.OnTapped = func() {
		cfg := guiConfig{
			server:   strings.TrimSpace(serverEntry.Text),
			vkey:     strings.TrimSpace(vKeyEntry.Text),
			connType: connSelect.Selected,
			verify:   verifyCheck.Checked,
			proxyUrl: strings.TrimSpace(proxyEntry.Text),
			user:     strings.TrimSpace(userEntry.Text),
			pass:     passEntry.Text,
		}
		if group.isRunning() {
			group.stop()
			statusLabel.SetText("已停止")
			startBtn.SetText("启动")
			return
		}
		if cfg.server == "" || cfg.vkey == "" {
			statusLabel.SetText("请填写服务端地址与 vkey")
			return
		}
		saveConfig(cfg)
		group.start(cfg, statusLabel)
		startBtn.SetText("停止")
	}

	return container.NewVBox(
		widget.NewLabel("Npc "+version.VERSION+"  —  TLS / Basic 认证 / 多服务端"),
		widget.NewForm(
			widget.NewFormItem("服务端", serverEntry),
			widget.NewFormItem("Vkey", vKeyEntry),
			widget.NewFormItem("连接类型", connSelect),
			widget.NewFormItem("TLS", verifyCheck),
			widget.NewFormItem("代理", proxyEntry),
			widget.NewFormItem("Basic 用户名", userEntry),
			widget.NewFormItem("Basic 密码", passEntry),
		),
		startBtn,
		statusLabel,
		logScroll,
	)
}

func (g *clientGroup) isRunning() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running
}

// start 为每个服务端各启一个客户端，各自独立重连；conn_type=tls 时按配置决定是否校验证书
func (g *clientGroup) start(cfg guiConfig, status *widget.Label) {
	servers := config.SplitServerAddr(cfg.server)
	if len(servers) == 0 {
		status.SetText("服务端地址无效")
		return
	}
	client.SetTlsVerify(cfg.verify)
	cnf := &config.Config{CommonConfig: &config.CommonConfig{
		VKey: cfg.vkey,
		Tp:   cfg.connType,
		Client: &file.Client{Cnf: &file.Config{
			U: cfg.user,
			P: cfg.pass,
		}},
	}}
	g.mu.Lock()
	g.running = true
	g.mu.Unlock()

	for _, serverAddr := range servers {
		go func(addr string) {
			for {
				if !g.isRunning() {
					return
				}
				cl := client.NewRPClient(addr, cfg.vkey, cfg.connType, cfg.proxyUrl, cnf, 60)
				g.mu.Lock()
				g.clients = append(g.clients, cl)
				g.mu.Unlock()
				status.SetText("已连接 " + addr)
				cl.Start()
				if !g.isRunning() {
					return
				}
				status.SetText("重连中 " + addr)
				logs.Warn("[%s] client closed, reconnecting in 5 seconds...", addr)
				time.Sleep(time.Second * 5)
			}
		}(serverAddr)
	}
	status.SetText(fmt.Sprintf("已启动，服务端 %s", strings.Join(servers, ", ")))
}

func (g *clientGroup) stop() {
	g.mu.Lock()
	g.running = false
	clients := g.clients
	g.clients = nil
	g.mu.Unlock()
	for _, cl := range clients {
		go cl.Close()
	}
}

// configPath 返回 GUI 配置文件路径，与 npc 命令行的 npc.conf 保持一致
func configPath() (string, error) {
	dir, err := getDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "npc.conf"), nil
}

func getDir() (dir string, err error) {
	if runtime.GOOS != "android" {
		dir, err = os.UserConfigDir()
		if err != nil {
			return
		}
		dir = path.Join(dir, "npc-gui")
	} else {
		dir = "/data/data/org.nps.client/files"
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return
	}
	return
}

// saveConfig 以 ini 形式保存，npc 命令行也能直接使用同一份配置
func saveConfig(cfg guiConfig) {
	filePath, err := configPath()
	if err != nil {
		logs.Warn("not found config dir")
		return
	}
	var b strings.Builder
	b.WriteString("[common]\n")
	b.WriteString("server_addr=" + cfg.server + "\n")
	b.WriteString("vkey=" + cfg.vkey + "\n")
	b.WriteString("conn_type=" + cfg.connType + "\n")
	b.WriteString("tls_verify=" + boolStr(cfg.verify) + "\n")
	if cfg.proxyUrl != "" {
		b.WriteString("proxy_url=" + cfg.proxyUrl + "\n")
	}
	if cfg.user != "" {
		b.WriteString("basic_username=" + cfg.user + "\n")
	}
	if cfg.pass != "" {
		b.WriteString("basic_password=" + cfg.pass + "\n")
	}
	if err := ioutil.WriteFile(filePath, []byte(b.String()), 0600); err != nil {
		logs.Error(err)
	}
}

// loadConfig 读取配置，兼容旧版本 GUI 的三行（地址 / vkey / 类型）格式
func loadConfig() guiConfig {
	cfg := guiConfig{connType: "tcp"}
	filePath, err := configPath()
	if err != nil {
		return cfg
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return cfg
	}
	content := strings.Replace(string(data), "\r\n", "\n", -1)
	if !strings.Contains(content, "=") {
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			cfg.server = strings.TrimSpace(lines[0])
		}
		if len(lines) > 1 {
			cfg.vkey = strings.TrimSpace(lines[1])
		}
		if len(lines) > 2 && strings.TrimSpace(lines[2]) != "" {
			cfg.connType = strings.TrimSpace(lines[2])
		}
		return cfg
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "server_addr":
			cfg.server = value
		case "vkey":
			cfg.vkey = value
		case "conn_type":
			cfg.connType = value
		case "tls_verify":
			cfg.verify = common.GetBoolByStr(value)
		case "proxy_url":
			cfg.proxyUrl = value
		case "basic_username":
			cfg.user = value
		case "basic_password":
			cfg.pass = value
		}
	}
	return cfg
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
