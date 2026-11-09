#!/usr/bin/env bash
# nps 服务端管理脚本
#   - 服务安装 / 启停 / 状态（调用 nps 自带 service 功能）
#   - 生成 TLS 证书（web 管理页与 bridge TLS 共用）
#   - 一键切换 bridge_type：tcp / kcp / tls
#
# 用法:
#   ./scripts/nps-server.sh run                    前台运行（调试）
#   ./scripts/nps-server.sh install                安装为系统服务
#   ./scripts/nps-server.sh start|stop|restart|status|uninstall|update
#   ./scripts/nps-server.sh gencert                生成 conf/server.pem / server.key
#   ./scripts/nps-server.sh tls on|off             切换 bridge 为 tls / tcp
#
# 环境变量:
#   NPS_BIN（默认 ./nps）  NPS_CONF（默认 conf/nps.conf）
#   NPS_CERT（默认 conf/server.pem）  NPS_KEY（默认 conf/server.key）
set -eu

cd "$(dirname "$0")/.."

NPS_BIN="${NPS_BIN:-./nps}"
CONF="${NPS_CONF:-conf/nps.conf}"
CERT="${NPS_CERT:-conf/server.pem}"
KEY="${NPS_KEY:-conf/server.key}"

usage() {
	sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
}

conf_get() {
	[ -f "$CONF" ] || return 0
	sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" "$CONF" | head -n 1 | tr -d '\r'
}

conf_set() {
	key="$1"
	value="$2"
	if [ ! -f "$CONF" ]; then
		echo "config file $CONF not found" >&2
		exit 1
	fi
	if grep -qE "^[[:space:]]*${key}[[:space:]]*=" "$CONF"; then
		sed -i.bak "s#^[[:space:]]*${key}[[:space:]]*=.*#${key}=${value}#" "$CONF"
		rm -f "${CONF}.bak"
	else
		printf '%s=%s\n' "$key" "$value" >>"$CONF"
	fi
}

gen_cert() {
	if ! command -v openssl >/dev/null 2>&1; then
		echo "openssl not found: nps will fall back to the built-in self-signed certificate"
		return 1
	fi
	host="$(conf_get web_host)"
	[ -n "$host" ] || host="nps.local"
	mkdir -p "$(dirname "$CERT")" "$(dirname "$KEY")"
	# 带 SAN 的证书：浏览器用 IP 访问、客户端开启 tls_verify 时都能校验通过
	if ! openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
		-keyout "$KEY" -out "$CERT" \
		-subj "/CN=${host}" \
		-addext "subjectAltName=DNS:${host},DNS:localhost,IP:127.0.0.1" >/dev/null 2>&1; then
		openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
			-keyout "$KEY" -out "$CERT" -subj "/CN=${host}" >/dev/null 2>&1
	fi
	chmod 600 "$KEY"
	echo "certificate generated: $CERT / $KEY"
}

tls_on() {
	gen_cert || true
	conf_set bridge_type tls
	conf_set bridge_cert_file "$CERT"
	conf_set bridge_key_file "$KEY"
	echo "bridge_type=tls written to $CONF"
	echo "clients must connect with: npc -type=tls ... (or conn_type=tls in npc.conf)"
}

tls_off() {
	conf_set bridge_type tcp
	echo "bridge_type=tcp written to $CONF"
}

cmd="${1:-}"
[ $# -gt 0 ] && shift || true

case "$cmd" in
run)
	exec "$NPS_BIN" "$@"
	;;
install | start | stop | restart | uninstall | update)
	exec "$NPS_BIN" "$cmd" "$@"
	;;
status)
	if "$NPS_BIN" status 2>/dev/null; then
		exit 0
	fi
	if pgrep -f "$(basename "$NPS_BIN")" >/dev/null 2>&1; then
		echo "running"
	else
		echo "stopped"
		exit 1
	fi
	;;
gencert)
	gen_cert
	;;
tls)
	case "${1:-on}" in
	on | enable | true) tls_on ;;
	off | disable | false) tls_off ;;
	*)
		echo "usage: $0 tls on|off" >&2
		exit 1
		;;
	esac
	;;
*)
	usage
	exit 1
	;;
esac
