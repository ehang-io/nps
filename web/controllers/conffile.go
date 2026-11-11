package controllers

// nps.conf 的就地改写：Web 后台修改密钥后写回配置文件，重启依然生效。
// 只做行级的 key=value 替换，注释、其它配置项与缩进全部原样保留。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ehang.io/nps/lib/common"
)

// confFilePath 返回 nps.conf 的绝对路径，与启动时 beego.LoadAppConfig 用的是同一个文件
func confFilePath() string {
	return filepath.Join(common.GetRunPath(), "conf", "nps.conf")
}

// updateConfValue 在配置文件里把 key 设为 value。
//   - 已存在（未注释）的同名配置就地替换，保留缩进与行尾符；
//   - 不存在则追加到文件末尾；
//   - 被注释掉的同名项不会被当成有效配置，会追加一行新的。
func updateConfValue(path, key, value string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("config key must not be empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(raw)
	lines := strings.Split(text, "\n")
	replaced := false
	for i, line := range lines {
		body := strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimLeft(body, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		name, _, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		indent := body[:len(body)-len(trimmed)]
		cr := ""
		if strings.HasSuffix(line, "\r") {
			cr = "\r"
		}
		lines[i] = indent + key + "=" + value + cr
		replaced = true
	}
	var out string
	if replaced {
		out = strings.Join(lines, "\n")
	} else {
		out = text
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += key + "=" + value + "\n"
	}
	return writeFileAtomic(path, []byte(out))
}

// writeFileAtomic 先写临时文件再 rename，避免进程被中断时留下半截配置
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".nps-conf-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
