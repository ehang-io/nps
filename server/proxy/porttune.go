package proxy

import (
	"net"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
)

// FirstByteGuardTimeout 返回某条 tcp 隧道应使用的首字节超时，0 表示不启用。
//
// 判定顺序：
//  1. 公网端口命中常见端口表，就用那张表（3389/22/80/443 等客户端先说话的协议）
//  2. 公网端口是随意选的（如 13389 -> 内网 3389），回退到内网目标端口判断
//  3. 命中「服务端先说话」的协议（FTP 21、MySQL 3306）一律跳过，否则会挡掉正常用户
//
// 需要彻底关闭时把 nps.conf 的 first_byte_timeout 设为 0。
func FirstByteGuardTimeout(task *file.Tunnel) time.Duration {
	if task == nil {
		return 0
	}
	profile := common.ProfileForPort(task.Port)
	if !profile.Known {
		profile = common.ProfileForPort(targetPort(task))
	}
	if !profile.Known || profile.ServerFirst {
		return 0
	}
	seconds := beego.AppConfig.DefaultInt("first_byte_timeout", 15)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// targetPort 从隧道目标里取出第一个端口，取不到返回 0
func targetPort(task *file.Tunnel) int {
	if task == nil || task.Target == nil {
		return 0
	}
	for _, addr := range strings.Fields(strings.ReplaceAll(task.Target.TargetStr, ",", " ")) {
		if _, portStr, err := net.SplitHostPort(strings.TrimSpace(addr)); err == nil {
			if p, err := strconv.Atoi(portStr); err == nil {
				return p
			}
		}
	}
	return 0
}
