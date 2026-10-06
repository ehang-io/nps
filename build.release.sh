#!/usr/bin/env bash
#
# Build the release artifacts, stamped with the release tag.
#
# This builds the handful of platforms this fleet actually runs, not the
# thirty-three of build.assets.sh. Add to PLATFORMS if that changes:
#
#   PLATFORMS="linux/amd64 linux/arm64 linux/arm" ./build.release.sh v0.26.11
#
# Asset names match the old scheme (linux_amd64_client.tar.gz) because both
# deploy/update.sh and the client's own updater derive the asset name from
# GOOS and GOARCH that way.
set -euo pipefail

TAG=${1:-dev}
OUT=${OUT:-dist}
PLATFORMS=${PLATFORMS:-"linux/amd64 linux/arm64"}

# The tag is stamped into VERSION, and the server decides whether a connected
# client is new enough to accept a pushed update by looking for this marker in
# the version it reported. A release tagged without it produces nodes the
# server will refuse to push to, which is a confusing thing to discover later.
FORK_MARKER=$(grep -oP 'ForkMarker = "\K[^"]+' lib/version/version.go)
if [[ $TAG != dev && $TAG != *"$FORK_MARKER"* ]]; then
    echo "ERROR: tag '$TAG' does not contain the fork marker '$FORK_MARKER'." >&2
    echo "       Tag releases like v0.27.0${FORK_MARKER}1 so pushed updates keep working." >&2
    exit 1
fi

# VERSION is only ever displayed and reported to the server, never compared,
# so the tag can go in verbatim. GetVersion, which the server does compare
# byte for byte, is deliberately left alone.
LDFLAGS="-s -w -X ehang.io/nps/lib/version.VERSION=${TAG}"

rm -rf "$OUT"
mkdir -p "$OUT"

for platform in $PLATFORMS; do
    goos=${platform%%/*}
    goarch=${platform##*/}
    label="${goos}_${goarch}"

    echo "==> ${label} client"
    CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
        go build -trimpath -ldflags "$LDFLAGS" -o npc ./cmd/npc/npc.go
    tar -czf "${OUT}/${label}_client.tar.gz" npc conf/npc.conf conf/multi_account.conf
    rm -f npc

    echo "==> ${label} server"
    CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
        go build -trimpath -ldflags "$LDFLAGS" -o nps ./cmd/nps/nps.go
    tar -czf "${OUT}/${label}_server.tar.gz" nps \
        conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json \
        conf/server.key conf/server.pem web/views web/static
    rm -f nps
done

# The checksum file is the one thing a curl-driven update can actually verify,
# so every asset has to be in it.
(cd "$OUT" && sha256sum ./*.tar.gz | sed 's|\./||' > sha256sums.txt)

echo
echo "==> ${OUT}/sha256sums.txt"
cat "${OUT}/sha256sums.txt"
