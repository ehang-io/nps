#!/usr/bin/env bash
#
# One command to replace a running npc or nps with a release build.
#
#   curl -fsSL https://github.com/hector918/nps/releases/latest/download/update.sh | sudo bash
#   sudo update.sh --tag v0.27.13-hz1     pin a release instead of taking the latest
#   sudo update.sh --file /tmp/npc       install a binary already on this host
#   sudo update.sh --rollback            put the previous binary back
#   DRY_RUN=1 sudo update.sh             show what would happen, change nothing
#
# It touches nothing but the binary, and on a server the web UI's templates and
# assets that ship with it. The service unit, its flags and any conf/ file are
# left exactly as they are, so a node started with -config= and a node started
# with bare flags need no special handling.
#
# Every release carries this script as an asset, which is where the command
# above takes it from. Never fetch it from master: piping it to a root shell
# hands every node to whoever can write to the branch it came from.
set -euo pipefail

REPO=${REPO:-hector918/nps}
WAIT_SECS=${WAIT_SECS:-30}
KEEP_BACKUPS=${KEEP_BACKUPS:-3}
DRY_RUN=${DRY_RUN:-0}

TAG=""
LOCAL_FILE=""
ROLLBACK=0

# Spelled out rather than read back out of the file: the usual way to run this
# is piped into bash, where the script is stdin and $0 is "bash".
usage() {
    cat <<'USAGE'
update.sh -- replace a running npc or nps with a release build

  curl -fsSL https://github.com/hector918/nps/releases/latest/download/update.sh | sudo bash
  ... | sudo bash -s -- --tag v0.27.13-hz1   install a specific release
  ... | sudo bash -s -- --rollback          put the previous binary back

  --tag TAG      release to install, default the latest one
  --file PATH    install a binary already on this host instead of downloading
  --rollback     restore the most recent backup
  -h, --help     this text

  REPO=owner/name   pull from a different repository
  WAIT_SECS=30      how long to wait for the tunnel to come back
  DRY_RUN=1         show what would happen, change nothing
USAGE
}

info() { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*"; }
die()  { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
run()  { if [[ $DRY_RUN == 1 ]]; then printf '  would run: %s\n' "$*"; else "$@"; fi; }

while [[ $# -gt 0 ]]; do
    case $1 in
        --tag)      TAG=${2:?--tag needs a value}; shift 2 ;;
        --file)     LOCAL_FILE=${2:?--file needs a path}; shift 2 ;;
        --rollback) ROLLBACK=1; shift ;;
        -h|--help)  usage; exit 0 ;;
        *)          die "unknown argument: $1" ;;
    esac
done

[[ $EUID -eq 0 ]] || die "must run as root"

# ---------------------------------------------------------------- locate ----
# The running process is the most reliable source of truth: its exe link gives
# the binary and its cgroup gives the unit, whatever that unit is called
# (Npc.service, npc.service, nps.service, ...).
ROLE=""
PID=""
for candidate in npc nps; do
    PID=$(pgrep -x "$candidate" | head -1 || true)
    if [[ -n $PID ]]; then ROLE=$candidate; break; fi
done

if [[ -n $PID ]]; then
    # A containerised node is visible from the host but must not be updated
    # this way: the path in /proc/PID/exe belongs to the container's mount
    # namespace, and a binary swapped into a container is undone the next time
    # it is recreated from its image. Say so plainly instead of failing later
    # with something that reads like a broken download.
    if grep -qE 'docker|containerd|kubepods|lxc|libpod' "/proc/$PID/cgroup" 2>/dev/null; then
        printf 'ERROR: %s here runs inside a container, not under a service manager.\n' "$ROLE" >&2
        printf '       Update it by rebuilding the image and recreating the container:\n' >&2
        printf '         docker inspect %s --format "{{json .Config}} {{json .HostConfig}}"   # keep these settings\n' "$ROLE" >&2
        printf '         docker build -f Dockerfile.%s --build-arg VERSION=<tag> -t <image>:<tag> .\n' "$ROLE" >&2
        printf '       Swapping the binary inside a running container would be undone on the next recreate.\n' >&2
        exit 1
    fi
    BIN=$(readlink "/proc/$PID/exe" | sed 's/ (deleted)$//')
    UNIT=$(grep -o '[^/]*\.service' "/proc/$PID/cgroup" | head -1 || true)
    [[ -n $UNIT ]] || die "$ROLE is running as pid $PID but not under any systemd unit; this script restarts through systemctl"
else
    info "neither npc nor nps is running, falling back to a unit lookup"
    UNIT=$(systemctl list-units --type=service --all --plain --no-legend 2>/dev/null \
           | awk '{print $1}' | grep -iE '^np[cs].*\.service$' | head -1 || true)
    [[ -n $UNIT ]] || die "cannot find an npc or nps service unit"
    BIN=$(systemctl show "$UNIT" -p ExecStart --value \
          | sed -n 's/.*path=\([^ ;]*\).*/\1/p' | head -1)
    ROLE=$(basename "${BIN:-}")
fi
[[ -n ${UNIT:-} ]]        || die "cannot determine the service unit"
[[ -n ${BIN:-} && -f $BIN ]] || die "cannot find the $ROLE binary (unit $UNIT)"

info "role:   $ROLE"
info "unit:   $UNIT"
info "binary: $BIN"

# A server renders its web UI from web/views and web/static beside its config,
# not from the binary, so a new binary alone keeps serving the old pages. The
# directory is found the way nps finds it (common.GetRunPath): /etc/nps when
# that exists, otherwise the directory of its argv[0] -- not of /proc/PID/exe,
# which resolves symlinks nps itself does not, and would name a different
# directory for a binary started through one.
WEB_DIR=""
if [[ $ROLE == nps ]]; then
    if [[ -d /etc/nps ]]; then
        RUN_DIR=/etc/nps
    elif [[ -n $PID ]]; then
        ARGV0=$(tr '\0' '\n' < "/proc/$PID/cmdline" | head -1)
        [[ $ARGV0 == /* ]] || ARGV0="$(readlink "/proc/$PID/cwd")/$ARGV0"
        RUN_DIR=$(dirname "$ARGV0")
    else
        RUN_DIR=$(dirname "$BIN")
    fi
    WEB_DIR="$RUN_DIR/web"
    # A backup counts too: it is what --rollback needs, even if the live
    # directory is missing.
    if [[ -d $WEB_DIR/views ]] || compgen -G "$WEB_DIR.bak.*" >/dev/null; then
        info "web:    $WEB_DIR"
    else
        info "WARNING: no web UI at $WEB_DIR, updating the binary only"
        WEB_DIR=""
    fi
fi

# -------------------------------------------------------------- rollback ----
if [[ $ROLLBACK == 1 ]]; then
    BACKUP=$(ls -1t "$BIN".bak.* 2>/dev/null | head -1 || true)
    [[ -n $BACKUP ]] || die "no backup found next to $BIN"
    info "restoring $BACKUP"
    run mv -f "$BACKUP" "$BIN"
    # The web UI backed up with that binary shares its timestamp. Restoring
    # one without the other would serve pages the binary may not handle.
    if [[ -n $WEB_DIR && -d $WEB_DIR.bak.${BACKUP##*.bak.} ]]; then
        info "restoring $WEB_DIR.bak.${BACKUP##*.bak.}"
        run rm -rf "$WEB_DIR"
        run mv -f "$WEB_DIR.bak.${BACKUP##*.bak.}" "$WEB_DIR"
    fi
    run systemctl restart "$UNIT"
    info "rolled back"
    exit 0
fi

# ------------------------------------------------------------------ fetch ---
# Staged beside the target binary, not in /tmp: the smoke test below executes
# it, and /tmp is mounted noexec on plenty of hardened hosts. It also keeps the
# final move a rename on one filesystem.
WORK=$(mktemp -d "$(dirname "$BIN")/.update.XXXXXX")
WEB_NEW=""
trap 'rm -rf "$WORK" ${WEB_NEW:+"$WEB_NEW"}' EXIT

if [[ -n $LOCAL_FILE ]]; then
    [[ -f $LOCAL_FILE ]] || die "no such file: $LOCAL_FILE"
    cp "$LOCAL_FILE" "$WORK/staged"
    info "using local file $LOCAL_FILE"
    if [[ -n $WEB_DIR ]]; then
        info "WARNING: --file installs the binary only, the web UI at $WEB_DIR is left as it is"
        WEB_DIR=""
    fi
else
    # These must be the GOARCH values build.release.sh labels its output with,
    # not finer-grained ones: asking for linux_arm_v7 when the builder produced
    # linux_arm just 404s.
    case "$(uname -m)" in
        x86_64)          ARCH=amd64 ;;
        aarch64|arm64)   ARCH=arm64 ;;
        armv7l|armv6l)   ARCH=arm ;;
        i386|i686)       ARCH=386 ;;
        *)               die "unsupported architecture: $(uname -m)" ;;
    esac
    KIND=client
    [[ $ROLE == nps ]] && KIND=server
    ASSET="linux_${ARCH}_${KIND}.tar.gz"

    if [[ -n $TAG ]]; then
        BASE="https://github.com/${REPO}/releases/download/${TAG}"
    else
        # This redirect always resolves to the newest release and, unlike the
        # API, is not rate limited per source IP -- which matters when many
        # nodes sit behind one address.
        BASE="https://github.com/${REPO}/releases/latest/download"
    fi

    info "fetching $BASE/$ASSET"
    curl -fsSL -o "$WORK/$ASSET" "$BASE/$ASSET" \
        || die "cannot download $ASSET (does the release exist?)"
    curl -fsSL -o "$WORK/sha256sums.txt" "$BASE/sha256sums.txt" \
        || die "cannot download sha256sums.txt"

    WANT=$(awk -v a="$ASSET" '$2 == a || $2 == "./" a {print $1}' "$WORK/sha256sums.txt" | head -1)
    [[ -n $WANT ]] || die "$ASSET is not listed in sha256sums.txt"
    GOT=$(sha256sum "$WORK/$ASSET" | cut -d' ' -f1)
    [[ $WANT == "$GOT" ]] || die "checksum mismatch: got $GOT, want $WANT"
    info "checksum ok"

    # Only the binary comes out. These tarballs also carry conf/npc.conf and,
    # for the server, web/views and web/static. Unpacking the config over a
    # live node would replace its vkey and every tunnel it serves with the
    # template, on every node at once.
    tar -xzOf "$WORK/$ASSET" "$ROLE" > "$WORK/staged" \
        || die "no $ROLE inside $ASSET"

    # The web UI is code, not configuration, and has to match the binary: it
    # is replaced whole. Local edits to the templates do not survive this.
    if [[ -n $WEB_DIR ]]; then
        tar -xzf "$WORK/$ASSET" -C "$WORK" web/views web/static \
            || die "no web/views or web/static inside $ASSET"
    fi
fi

chmod 0755 "$WORK/staged"

# ---------------------------------------------------------------- verify ----
# Read the ELF magic directly rather than shelling out to file(1), which is
# not installed on a minimal server image -- and whose absence made this look
# like a corrupt download rather than a missing tool.
if [[ $(od -An -tx1 -N4 < "$WORK/staged" | tr -d ' \n') != 7f454c46 ]]; then
    die "the staged file is not an ELF binary"
fi
if cmp -s "$WORK/staged" "$BIN"; then
    # The pages can lag a matching binary -- after --file, or an update that
    # predates this script handling them -- so only stop if they match too.
    if [[ -z $WEB_DIR ]] || { diff -rq "$WORK/web/views" "$WEB_DIR/views" &&
                              diff -rq "$WORK/web/static" "$WEB_DIR/static"; } >/dev/null 2>&1; then
        info "already running this exact binary, nothing to do"
        exit 0
    fi
    info "the binary is current but the web UI is not, updating it"
fi

# -version prints and exits on both npc and nps. Never use a bare word here:
# npc treats an unrecognised argument as "start the service".
if ! STAGED_VERSION=$("$WORK/staged" -version 2>&1 | head -1); then
    die "the staged binary failed its smoke test: $STAGED_VERSION"
fi
info "staged: $STAGED_VERSION"
info "current: $("$BIN" -version 2>/dev/null | head -1 || echo unknown)"

# --------------------------------------------------------------- install ----
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$BIN.bak.$STAMP"
info "backing up to $BACKUP"
run cp -a "$BIN" "$BACKUP"
# The new web UI is assembled in full beside the live one before anything is
# changed, so a failure here -- a full disk, say -- leaves the node exactly as
# it was. The swap further down is then only renames.
WEB_BACKUP=""
if [[ -n $WEB_DIR ]]; then
    WEB_NEW="$WEB_DIR.new"
    WEB_BACKUP="$WEB_DIR.bak.$STAMP"
    info "staging the web UI in $WEB_NEW"
    run rm -rf "$WEB_NEW"
    if [[ -d $WEB_DIR ]]; then
        run cp -a "$WEB_DIR" "$WEB_NEW"
        run rm -rf "$WEB_NEW/views" "$WEB_NEW/static"
    else
        run mkdir -p "$WEB_NEW"
    fi
    run cp -a "$WORK/web/views" "$WORK/web/static" "$WEB_NEW/"
fi

# A running executable cannot be overwritten (ETXTBSY) but its directory entry
# can be replaced: stage alongside, then rename.
run install -m 0755 "$WORK/staged" "$BIN.new"
# chown --reference is GNU-only; fall back to whatever stat can tell us.
OWNER=$(stat -c '%u:%g' "$BACKUP" 2>/dev/null || true)
[[ -n $OWNER ]] && run chown "$OWNER" "$BIN.new"
run mv -f "$BIN.new" "$BIN"

if [[ -n $WEB_DIR ]]; then
    info "replacing the web UI, previous one kept as $WEB_BACKUP"
    [[ -d $WEB_DIR ]] && run mv "$WEB_DIR" "$WEB_BACKUP"
    run mv "$WEB_NEW" "$WEB_DIR"
fi

info "restarting $UNIT"
run systemctl restart "$UNIT"

if [[ $DRY_RUN == 1 ]]; then
    info "dry run complete, nothing was changed"
    exit 0
fi

# ---------------------------------------------------------------- prove -----
# Readiness differs by role and neither form depends on the node's log level.
# A client proves itself by connecting out to its server; a server proves
# itself by listening again.
#
# The socket check degrades rather than failing closed. A missing ss would
# otherwise make every healthy update look dead and get rolled back, which is
# a far worse outcome than proving less.
if command -v ss >/dev/null 2>&1; then
    SOCKET_TOOL=ss
elif command -v netstat >/dev/null 2>&1; then
    SOCKET_TOOL=netstat
else
    SOCKET_TOOL=""
    info "WARNING: no ss or netstat here, falling back to a liveness check"
fi

sockets_ready() {
    local pid=$1
    case $SOCKET_TOOL in
        ss)
            if [[ $ROLE == nps ]]; then
                ss -tlnp 2>/dev/null | grep -q "pid=$pid,"
            else
                # TCP for the usual bridge, UDP for a node whose bridge is
                # kcp -- checking only TCP would roll back every healthy kcp
                # node, every time.
                ss -tnp state established 2>/dev/null | grep -q "pid=$pid," ||
                    ss -unp 2>/dev/null | grep -q "pid=$pid,"
            fi
            ;;
        netstat)
            if [[ $ROLE == nps ]]; then
                netstat -tlnp 2>/dev/null | grep -q "[[:space:]]$pid/"
            else
                netstat -tunp 2>/dev/null | grep -q "[[:space:]]$pid/"
            fi
            ;;
        *)
            return 1
            ;;
    esac
}

info "waiting up to ${WAIT_SECS}s for $ROLE to come back"
ok=0
stable=0
deadline=$((SECONDS + WAIT_SECS))
while ((SECONDS < deadline)); do
    if systemctl is-active --quiet "$UNIT"; then
        NEW_PID=$(pgrep -x "$ROLE" | head -1 || true)
        if [[ -n $NEW_PID ]]; then
            if [[ -n $SOCKET_TOOL ]]; then
                sockets_ready "$NEW_PID" && { ok=1; break; }
            else
                # Without a socket tool, the strongest available evidence is
                # that one pid stayed up across the settle window: a binary
                # that cannot run at all dies inside it.
                stable=$((stable + 1))
                ((stable >= 8)) && { ok=1; break; }
            fi
        else
            stable=0
        fi
    else
        stable=0
    fi
    sleep 1
done

if ((ok == 0)); then
    info "FAILED to come up, rolling back"
    mv -f "$BACKUP" "$BIN"
    if [[ -n $WEB_BACKUP && -d $WEB_BACKUP ]]; then
        rm -rf "$WEB_DIR"
        mv -f "$WEB_BACKUP" "$WEB_DIR"
    fi
    systemctl restart "$UNIT"
    sleep 3
    systemctl is-active --quiet "$UNIT" \
        && die "rolled back, this node is up on the previous binary" \
        || die "rolled back but $UNIT is still not active, needs a look"
fi

# --------------------------------------------------------------- cleanup ----
# A plain read loop rather than mapfile, which needs bash 4 and is one more
# thing to be missing on a stripped-down host.
ls -1t "$BIN".bak.* 2>/dev/null | tail -n +$((KEEP_BACKUPS + 1)) | while read -r f; do
    [[ -n $f ]] && rm -f "$f" && info "pruned $f"
done
if [[ -n $WEB_DIR ]]; then
    ls -1dt "$WEB_DIR".bak.* 2>/dev/null | tail -n +$((KEEP_BACKUPS + 1)) | while read -r f; do
        [[ -n $f ]] && rm -rf "$f" && info "pruned $f"
    done
fi

info "done. $ROLE is up on $STAGED_VERSION"
