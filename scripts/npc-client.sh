#!/usr/bin/env bash
# npc 客户端管理脚本（nps -server 连接参数统一入口）
#
# 用法:
#   ./scripts/npc-client.sh run     --server a.example.com:8024,b.example.com:8024 --vkey 123 [选项]
#   ./scripts/npc-client.sh install --server a.example.com:8024 --vkey 123 [选项]   # 安装为系统服务
#   ./scripts/npc-client.sh start|stop|restart|status|uninstall
#
# 选项:
#   --server/-s    服务端地址，多个用逗号分隔（多服务端穿透）
#   --vkey/-k      服务端 vkey
#   --type/-t      连接类型 tcp|kcp|tls（需与服务端 bridge_type 一致）
#   --tls-verify   type=tls 时校验服务端证书（自签证书请勿开启）
#   --proxy/-p     代理地址，如 socks5://user:pass@127.0.0.1:1080
#   --config/-c    使用配置文件（可写 basic_username / basic_password 等）
#   --user/--pass  可选：Basic 认证用户名 / 密码（配合服务端 public_vkey 模式）
#   其余参数原样透传给 npc
#
# 环境变量: NPC_BIN（默认 ./npc）
set -eu

cd "$(dirname "$0")/.."

NPC_BIN="${NPC_BIN:-./npc}"

usage() {
	sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'
}

action="run"
if [ $# -gt 0 ]; then
	case "$1" in
	run | install | start | stop | restart | status | uninstall | update)
		action="$1"
		shift
		;;
	esac
fi

server=""
vkey=""
conn=""
verify=""
proxy=""
config=""
user=""
pass=""
extra=()

while [ $# -gt 0 ]; do
	case "$1" in
	--server | -s)
		server="$2"
		shift 2
		;;
	--vkey | -k)
		vkey="$2"
		shift 2
		;;
	--type | -t)
		conn="$2"
		shift 2
		;;
	--tls-verify)
		verify="true"
		shift
		;;
	--proxy | -p)
		proxy="$2"
		shift 2
		;;
	--config | -c)
		config="$2"
		shift 2
		;;
	--user)
		user="$2"
		shift 2
		;;
	--pass)
		pass="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		extra+=("$1")
		shift
		;;
	esac
done

args=()
[ -n "$server" ] && args+=("-server=${server}")
[ -n "$vkey" ] && args+=("-vkey=${vkey}")
[ -n "$conn" ] && args+=("-type=${conn}")
[ -n "$verify" ] && args+=("-tls_verify=${verify}")
[ -n "$proxy" ] && args+=("-proxy=${proxy}")

# basic 凭据只能通过配置文件下发，这里就地写入指定的配置文件
if [ -n "$user" ] || [ -n "$pass" ]; then
	config="${config:-${NPC_CONF:-conf/npc.conf}}"
	if [ ! -f "$config" ]; then
		echo "config file $config not found, copy conf/npc.conf first" >&2
		exit 1
	fi
	set_kv() {
		key="$1"
		value="$2"
		if grep -qE "^[[:space:]]*${key}[[:space:]]*=" "$config"; then
			sed -i.bak "s#^[[:space:]]*${key}[[:space:]]*=.*#${key}=${value}#" "$config"
			rm -f "${config}.bak"
		else
			printf '%s=%s\n' "$key" "$value" >>"$config"
		fi
	}
	[ -n "$server" ] && set_kv server_addr "$server"
	[ -n "$vkey" ] && set_kv vkey "$vkey"
	[ -n "$conn" ] && set_kv conn_type "$conn"
	[ -n "$verify" ] && set_kv tls_verify "$verify"
	set_kv basic_username "$user"
	set_kv basic_password "$pass"
	args=("-config=${config}")
fi

if [ ${#extra[@]} -gt 0 ]; then
	args+=("${extra[@]}")
fi

case "$action" in
run)
	if [ ${#args[@]} -eq 0 ]; then
		usage
		exit 1
	fi
	exec "$NPC_BIN" "${args[@]}"
	;;
install)
	exec "$NPC_BIN" install "${args[@]}"
	;;
start | stop | restart | uninstall | update)
	exec "$NPC_BIN" "$action"
	;;
status)
	if [ -n "$config" ] && [ -f "$config" ]; then
		exec "$NPC_BIN" status "${args[@]}"
	fi
	exec "$NPC_BIN" status
	;;
esac
