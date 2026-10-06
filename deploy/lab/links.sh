#!/usr/bin/env bash
#
# Secret links against a link with a real delay: two containers with tc netem
# (42 ms each way), the current server and visitor, and the previous release's
# server for the case of an older server.
#
#   1. a new local connection on a kept link against one of its own
#   2. the visitor holds a steady number of connections to the server while
#      it carries many local ones
#   3. parallel large downloads over the links arrive intact
#   4. a link that has been idle for a minute is still there
#   5. the server restarts: the visitor comes back, and the links with it
#   6. an older server: connections of their own, nothing broken
#
#   deploy/lab/links.sh [old-tag]      old-tag defaults to v0.27.12-hz1
set -uo pipefail
cd "$(dirname "$0")/../.."
OLD_TAG=${1:-v0.27.12-hz1}
B=${LAB_DIR:-$(mktemp -d)}
echo "lab directory: $B"
mkdir -p "$B"/{old,new,conf,www,web}
OLD_SRC=$(mktemp -d)
cleanup() {
    docker rm -f lk-srv lk-cli >/dev/null 2>&1
    docker network rm lk-net >/dev/null 2>&1
    git worktree remove --force "$OLD_SRC/src" >/dev/null 2>&1
}
trap cleanup EXIT
git worktree add -q --detach "$OLD_SRC/src" "$OLD_TAG" || exit 1
(cd "$OLD_SRC/src" && CGO_ENABLED=0 go build -o "$B/old/nps" ./cmd/nps/nps.go && CGO_ENABLED=0 go build -o "$B/old/npc" ./cmd/npc/npc.go) || exit 1
git worktree remove --force "$OLD_SRC/src"
CGO_ENABLED=0 go build -o "$B/new/nps" ./cmd/nps/nps.go && CGO_ENABLED=0 go build -o "$B/new/npc" ./cmd/npc/npc.go &&
    (cd deploy/lab/labtool && CGO_ENABLED=0 go build -o "$B/labtool" .) || exit 1
cp -r web/views web/static "$B/web/"
cp conf/nps.conf conf/server.key conf/server.pem conf/multi_account.conf "$B/conf/"
: >"$B/conf/tasks.json"; : >"$B/conf/hosts.json"
for i in "1 tk-0123456789abcdef target" "2 vk-0123456789abcdef visitor"; do set -- $i
    printf '{"Cnf":{"U":"","P":"","Compress":false,"Crypt":false},"Id":%s,"VerifyKey":"%s","Addr":"","Remark":"%s","Status":true,"IsConnect":false,"RateLimit":0,"Flow":{"ExportFlow":0,"InletFlow":0,"FlowLimit":0},"NoStore":false,"NoDisplay":false,"MaxConn":0,"NowConn":0,"WebUserName":"","WebPassword":"","ConfigConnAllow":true,"MaxTunnelNum":0,"Version":""}\n*#*\n' "$1" "$2" "$3"
done >"$B/conf/clients.json"
echo "hello over a kept link" >"$B/www/index.html"
head -c 8000000 /dev/urandom >"$B/www/big.bin"; md5sum "$B/www/big.bin" | cut -c1-32 >"$B/big.md5"

docker build -q -t lab-netem - >/dev/null <<'DOCKER' || exit 1
FROM alpine:3.20
RUN apk add --no-cache iproute2-tc
DOCKER
docker rm -f lk-srv lk-cli >/dev/null 2>&1; docker network rm lk-net >/dev/null 2>&1
docker network create --internal lk-net >/dev/null || exit 1
for n in lk-srv lk-cli; do
    docker run -d --name $n --network lk-net --cap-add NET_ADMIN -v "$B":/lab:ro lab-netem sleep infinity >/dev/null
    docker exec $n tc qdisc replace dev eth0 root netem delay 42ms || exit 1
done
S() { docker exec lk-srv sh -c "$1"; }
C() { docker exec lk-cli sh -c "$1"; }

srv_start() { # srv_start <old|new>
    S 'for p in $(pidof nps); do kill $p; done; sleep 1'
    S 'mkdir -p /run/s && cp -r /lab/conf /lab/web /run/s/ 2>/dev/null; true'
    S "cp /lab/$1/nps /run/s/nps"
    docker exec -d lk-srv sh -c 'cd /run/s && exec ./nps >/run/s/nps.log 2>&1'
    sleep 3
}
cli_start() { # cli_start <npc binary dir>
    C 'for p in $(pidof npc); do kill $p; done; sleep 1'
    C "mkdir -p /run/c && cp /lab/$1/npc /run/c/npc
cat >/run/c/target.conf <<CONF
[common]
server_addr=lk-srv:8024
vkey=tk-0123456789abcdef
conn_type=tcp
auto_reconnection=true

[secret_t]
mode=secret
password=pw
target_addr=127.0.0.1:8090
CONF
cat >/run/c/visitor.conf <<CONF
[common]
server_addr=lk-srv:8024
vkey=vk-0123456789abcdef
conn_type=tcp
auto_reconnection=true

[secret_links]
password=pw
local_port=11434

[secret_own]
password=pw
local_port=11435
links=off
CONF"
    docker exec -d lk-cli sh -c 'cd /run/c && exec ./npc -config=/run/c/target.conf -debug=true >/run/c/target.log 2>&1'
    sleep 2
    docker exec -d lk-cli sh -c 'cd /run/c && exec ./npc -config=/run/c/visitor.conf -debug=true >/run/c/visitor.log 2>&1'
    sleep 6
}
conns() { S "netstat -tn 2>/dev/null | grep ':8024 ' | grep -c ESTABLISHED"; }
median() { sed -n 's/^median \([0-9]*\) ms.*/\1/p'; }

docker exec -d lk-cli /lab/labtool web /lab/www 127.0.0.1:8090
RESULT=()
note() { RESULT+=("$(printf '%-70s %s' "$1" "$2")"); }

echo; echo "=== 1+2. new server, new visitor: a connection on a link, against one of its own"
srv_start new; cli_start new
C 'rm -f /tmp/f; /lab/labtool fresh http://127.0.0.1:11434/ 3 >/dev/null'   # warm up
before=$(conns)
L=$(C '/lab/labtool fresh http://127.0.0.1:11434/ 12' | median)
O=$(C '/lab/labtool fresh http://127.0.0.1:11435/ 12' | median)
after=$(conns)
echo "  fresh connection, first byte: on a link ${L} ms, a connection of its own ${O} ms"
echo "  connections the server holds: ${before} before, ${after} after 24 local connections"
note "new local connection, first byte, on a link / of its own (ms)" "$L / $O"
note "server's connections before and after 24 local connections" "$before / $after"

echo; echo "=== 3. ten parallel 8 MB downloads over the links"
C 'for i in 1 2 3 4 5 6 7 8 9 10; do (wget -qO- http://127.0.0.1:11434/big.bin | md5sum | cut -c1-32 > /tmp/o$i) & done; wait'
sums=$(C 'cat /tmp/o* | sort | uniq -c')
echo "$sums" | sed 's/^/  /'
[[ $(echo "$sums" | wc -l) -eq 1 && $(echo "$sums" | awk '{print $2}') == "$(cat "$B/big.md5")" ]] && note "parallel downloads intact" ok || note "parallel downloads intact" FAIL

echo; echo "=== 4. idle for 70 s, then a connection"
sleep 70
L2=$(C '/lab/labtool fresh http://127.0.0.1:11434/ 3' | median)
echo "  first byte after the idle: ${L2} ms (on a link ${L})"
[[ -n $L2 && $L2 -le $((L + 60)) ]] && note "links still up after 70 s idle (first byte ms)" "ok $L2" || note "links still up after 70 s idle (first byte ms)" "FAIL ${L2:-none}"

echo; echo "=== 5. the server restarts"
T0=$(date +%s)
srv_start new
until C '/lab/labtool fresh http://127.0.0.1:11434/ 1' >/dev/null 2>&1; do sleep 1; [[ $(( $(date +%s) - T0 )) -gt 90 ]] && break; done
T1=$(( $(date +%s) - T0 ))
echo "  the first connection got through ${T1} s after the restart began"
sleep 10
L3=$(C '/lab/labtool fresh http://127.0.0.1:11434/ 6' | median)
echo "  ten seconds later, first byte: ${L3} ms (on a link ${L}, of its own ${O})"
[[ -n $L3 && $L3 -le $((L + 60)) ]] && note "links back after a server restart (first byte ms, seconds to first success)" "ok $L3, ${T1}s" || note "links back after a server restart (first byte ms, seconds to first success)" "FAIL ${L3:-none}"

echo; echo "=== 6. an older server"
srv_start old; cli_start new
sleep 8
OLD=$(C '/lab/labtool fresh http://127.0.0.1:11434/ 6' | median)
echo "  first byte, through the older server: ${OLD} ms (a connection of its own, as before)"
echo "  what the visitor says:"; C "grep -a -i 'secret links\|secret link' /run/c/visitor.log | tail -2 | cut -c1-200" | sed 's/^/    /'
[[ -n $OLD ]] && note "older server: connections work, of their own (first byte ms)" "ok $OLD" || note "older server: connections work, of their own (first byte ms)" FAIL

echo; echo "================ summary"
printf '%s\n' "${RESULT[@]}"
