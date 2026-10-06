#!/usr/bin/env bash
#
# Two-container lab for the bridge protocol break: one container is the server
# (nps), the other is the client side (the target npc with its web service,
# and the visitor npc that reaches it through a secret tunnel). They sit on a
# docker network with no way out, and the client container answers for
# github.com so that the self-updater can be exercised too.
#
# It answers the questions of a cutover: which combinations of old and new
# talk, what an updated or non-updated side looks like from the other,
# whether a pushed update to the incompatible release rolls itself back, and
# whether the order "clients first, then the server" comes out working.
#
#   deploy/lab/run.sh [old-tag]      old-tag defaults to v0.27.11-hz1
#
# Needs docker and go. Nothing touches the host's nps, npc or /etc/nps.
set -uo pipefail
cd "$(dirname "$0")/../.."
OLD_TAG=${1:-v0.27.11-hz1}
NEW_VER=v0.27.12-hz1-lab
B=${LAB_DIR:-$(mktemp -d)}
echo "lab directory: $B"
mkdir -p "$B"/{old,new,conf,www,release,web}

echo "== building"
build() { # build <srcdir> <outdir> <version-stamp or empty>
    local ld=""
    [[ -n $3 ]] && ld="-X ehang.io/nps/lib/version.VERSION=$3"
    (cd "$1" && CGO_ENABLED=0 go build -ldflags "$ld" -o "$2/nps" ./cmd/nps/nps.go &&
        CGO_ENABLED=0 go build -ldflags "$ld" -o "$2/npc" ./cmd/npc/npc.go) || exit 1
}
OLD_SRC=$(mktemp -d)
git worktree add -q --detach "$OLD_SRC/src" "$OLD_TAG" || exit 1
build "$OLD_SRC/src" "$B/old" ""
build "$PWD" "$B/new" "$NEW_VER"
git worktree remove --force "$OLD_SRC/src"
(cd deploy/lab/labtool && CGO_ENABLED=0 go build -o "$B/labtool" .) || exit 1
cp -r web/views web/static "$B/web/"
cp conf/nps.conf conf/server.key conf/server.pem conf/multi_account.conf "$B/conf/"
: >"$B/conf/tasks.json"; : >"$B/conf/hosts.json"
for i in "1 tk target" "2 vk visitor"; do set -- $i
    printf '{"Cnf":{"U":"","P":"","Compress":false,"Crypt":false},"Id":%s,"VerifyKey":"%s","Addr":"","Remark":"%s","Status":true,"IsConnect":false,"RateLimit":0,"Flow":{"ExportFlow":0,"InletFlow":0,"FlowLimit":0},"NoStore":false,"NoDisplay":false,"MaxConn":0,"NowConn":0,"WebUserName":"","WebPassword":"","ConfigConnAllow":true,"MaxTunnelNum":0,"Version":""}\n*#*\n' "$1" "$2" "$3"
done >"$B/conf/clients.json"
echo "hello from the target" >"$B/www/index.html"
# the release the self-updater will find: the new npc, stamped
tar czf "$B/release/linux_amd64_client.tar.gz" -C "$B/new" npc
(cd "$B/release" && sha256sum linux_amd64_client.tar.gz >sha256sums.txt)

echo "== containers"
docker rm -f lab-srv lab-cli >/dev/null 2>&1
docker network rm lab-net >/dev/null 2>&1
docker network create --internal lab-net >/dev/null || exit 1
docker run -d --name lab-srv --network lab-net -v "$B":/lab:ro alpine:3.20 sleep infinity >/dev/null
docker run -d --name lab-cli --network lab-net --add-host github.com:127.0.0.1 -v "$B":/lab:ro alpine:3.20 sleep infinity >/dev/null
docker exec -d lab-cli /lab/labtool web /lab/www 127.0.0.1:8090
trap 'docker rm -f lab-srv lab-cli >/dev/null 2>&1; docker network rm lab-net >/dev/null 2>&1' EXIT

S() { docker exec lab-srv sh -c "$1"; }
C() { docker exec lab-cli sh -c "$1"; }

srv_stop() { S 'for p in $(pidof nps); do kill $p; done; sleep 1'; }
cli_stop() { C 'for p in $(pidof npc); do kill $p; done; sleep 1'; }

# srv_start <old|new>: fresh state, unless KEEP is set
srv_start() {
    srv_stop
    [[ -n ${KEEP:-} ]] || S 'rm -rf /run/s; mkdir -p /run/s && cp -r /lab/conf /lab/web /run/s/'
    S "cp /lab/$1/nps /run/s/nps"
    docker exec -d lab-srv sh -c 'cd /run/s && exec ./nps >/run/s/nps.log 2>&1'
    sleep 3
}
# cli_start <old|new>; SRV overrides the address the clients dial
cli_start() {
    cli_stop
    local srv=${SRV:-lab-srv:8024}
    C "mkdir -p /run/c/target /run/c/visitor && cp /lab/$1/npc /run/c/target/npc && cp /lab/$1/npc /run/c/visitor/npc
cat >/run/c/target.conf <<CONF
[common]
server_addr=$srv
vkey=tk
conn_type=tcp
auto_reconnection=true

[secret_t]
mode=secret
password=pw
target_addr=127.0.0.1:8090
CONF
cat >/run/c/visitor.conf <<CONF
[common]
server_addr=$srv
vkey=vk
conn_type=tcp
auto_reconnection=true

[secret_t]
password=pw
local_port=11434
CONF"
    docker exec -d -e SSL_CERT_FILE=/run/github.pem lab-cli sh -c 'cd /run/c/target && exec ./npc -config=/run/c/target.conf -debug=true >/run/c/target.log 2>&1'
    sleep 2
    docker exec -d lab-cli sh -c 'cd /run/c/visitor && exec ./npc -config=/run/c/visitor.conf -debug=true >/run/c/visitor.log 2>&1'
    sleep 4
}
fetch() { C "/lab/labtool get http://127.0.0.1:11434/index.html $1" ; }
# works <n>: 0 when all n requests got through
works() { fetch "${1:-3}" >/tmp/lab-get.txt 2>&1; local rc=$?; sed 's/^/    /' /tmp/lab-get.txt; return $rc; }
tail_log() { # tail_log <container> <file> <pattern>
    docker exec "$1" sh -c "grep -a -i '$3' $2 | tail -${4:-3} | cut -c1-220" | sed 's/^/    /'
}

RESULT=()
verdict() { # verdict <name> <expected: pass|fail> <rc of works>
    local got=pass; [[ $3 -ne 0 ]] && got=fail
    local mark=OK; [[ $got != "$2" ]] && mark="UNEXPECTED"
    RESULT+=("$(printf '%-52s expect %-4s got %-4s %s' "$1" "$2" "$got" "$mark")")
}

echo; echo "=== 1. old server + old clients (baseline)"
KEEP= srv_start old; cli_start old
works 3; verdict "old server + old clients" pass $?

echo; echo "=== 2. new server + new clients, nothing to configure but the vkey"
KEEP= srv_start new; cli_start new
works 3; verdict "new server + new clients" pass $?

echo; echo "=== 3. OLD server + NEW clients (clients updated first, server not yet)"
KEEP= srv_start old; cli_start new; sleep 8
works 2; verdict "old server + new clients" fail $?
echo "  what the new client says:"; tail_log lab-cli /run/c/target.log 'error\|fail\|refus\|presented\|eof\|reset\|tls' 3
echo "  what the old server says:"; tail_log lab-srv /run/s/nps.log 'client\|error\|version' 3
echo "  new client still retrying? $(C 'pidof npc | wc -w') npc processes alive"

echo; echo "=== 4. NEW server + OLD clients (server updated first)"
KEEP= srv_start new; cli_start old; sleep 8
works 2; verdict "new server + old clients" fail $?
echo "  what the old client says:"; tail_log lab-cli /run/c/target.log 'error\|fail\|refus\|eof\|timeout' 3
echo "  what the new server says:"; tail_log lab-srv /run/s/nps.log 'TLS\|handshake\|error' 3
echo "  old client still retrying? $(C 'pidof npc | wc -w') npc processes alive"

echo; echo "=== 5. pushed update to the incompatible release, old server (does it roll back?)"
KEEP= srv_start old
docker exec -d lab-cli /lab/labtool github /lab/release /run/github.pem; sleep 1
cli_start old
echo "  target client version before: $(C '/run/c/target/npc -version | head -1')"
S "/lab/labtool push http://127.0.0.1:8080 admin 123 1 $NEW_VER" 2>&1 | sed 's/^/    /'
for t in 5 10 20; do
    sleep 5
    echo "  +${t}s version on disk: $(C '/run/c/target/npc -version | head -1')  | $(C 'grep -a -c selfupdate /run/c/target.log') selfupdate lines"
done
echo "  waiting out the verification window (60s) for the rollback..."
sleep 60
echo "  after the window, version on disk: $(C '/run/c/target/npc -version | head -1')"
tail_log lab-cli /run/c/target.log 'selfupdate' 6
sleep 8
works 3; verdict "pushed incompatible update rolled back, tunnel works" pass $?
C 'kill $(pgrep -f "labtool github")' >/dev/null 2>&1

echo; echo "=== 6. cutover in the order: clients first, then the server"
KEEP= srv_start old; cli_start old
echo "  working before the cutover:"; works 2
echo "  step 1: restart every client on the new binary (they go dark)"
cli_start new
echo "    through the tunnel now (server still old):"; works 1 && echo "    (unexpectedly worked)"
echo "  step 2: switch the server"
T0=$(date +%s)
KEEP=1 srv_start new
for i in $(seq 1 40); do
    if C "/lab/labtool get http://127.0.0.1:11434/index.html 1" >/dev/null 2>&1; then
        echo "    the tunnel is back $(( $(date +%s) - T0 ))s after the server switch started"; break
    fi
    sleep 1
done
works 3; verdict "cutover: clients, then server" pass $?

echo; echo "=== 7. a man in the middle that relays every byte between a client and the real server"
KEEP= srv_start new
docker exec -d lab-cli /lab/labtool mitm 127.0.0.1:9024 lab-srv:8024; sleep 1
SRV=127.0.0.1:9024 cli_start new; sleep 6
works 2; verdict "relaying man in the middle is refused" fail $?
echo "  what the client says:"; tail_log lab-cli /run/c/target.log 'did not accept\|alert\|relaying' 2
echo "  what the server says:"; tail_log lab-srv /run/s/nps.log 'ALERT' 2
A=$(S "/lab/labtool alerts http://127.0.0.1:8080 admin 123")
if echo "$A" | grep -q '"unknown-key"'; then echo "    the alert is on the server's /stats/alerts"; RESULT+=("$(printf '%-52s expect %-4s got %-4s %s' 'alert raised for the relayed session' yes yes OK)"); else echo "$A" | head -5; RESULT+=("$(printf '%-52s expect %-4s got %-4s %s' 'alert raised for the relayed session' yes no UNEXPECTED)"); fi
C 'kill $(pgrep -f "labtool mitm")' >/dev/null 2>&1

echo; echo "=== 8. the same vkey from a second address"
KEEP= srv_start new; cli_start new
docker exec -d lab-srv sh -c 'cd /run/s && exec /lab/new/npc -server=127.0.0.1:8024 -vkey=tk -debug=true >/run/s/intruder.log 2>&1'
sleep 6
A=$(S "/lab/labtool alerts http://127.0.0.1:8080 admin 123")
echo "  what the server says:"; tail_log lab-srv /run/s/nps.log 'ALERT' 2
if echo "$A" | grep -q '"duplicate"'; then RESULT+=("$(printf '%-52s expect %-4s got %-4s %s' 'duplicate vkey raised an alert' yes yes OK)"); else RESULT+=("$(printf '%-52s expect %-4s got %-4s %s' 'duplicate vkey raised an alert' yes no UNEXPECTED)"); fi
S 'for p in $(pidof npc); do kill $p; done' >/dev/null 2>&1

echo; echo "================ summary"
printf '%s\n' "${RESULT[@]}"
