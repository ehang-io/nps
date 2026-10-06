#!/usr/bin/env bash
#
# Point every version reference in the tree at a release tag.
#
#   ./deploy/bump-version.sh v0.27.4-hz1
#   git commit -am "release v0.27.4-hz1"
#   git tag v0.27.4-hz1
#   git push origin master v0.27.4-hz1
#   # the tag push builds and publishes the release, see release.yml
#
# Run it before tagging, not after: the tag captures the tree as it is at that
# commit, so docs fixed afterwards are not in the release anyone reads.
#
# The install one-liner in the READMEs no longer names a tag: it takes the
# script from the latest release's assets. What is left to keep current is
# the examples and the docker image tags, which do name a release, and the
# version a build from source reports.
set -euo pipefail

TAG=${1:-}
if [[ -z $TAG ]]; then
    echo "usage: $0 vX.Y.Z-hzN" >&2
    exit 1
fi

cd "$(dirname "$0")/.."

MARKER=$(grep -oP 'ForkMarker = "\K[^"]+' lib/version/version.go)
if [[ $TAG != *"$MARKER"* ]]; then
    echo "ERROR: tag '$TAG' does not contain the fork marker '$MARKER'." >&2
    echo "       The server only pushes updates to clients reporting it, and" >&2
    echo "       build.release.sh refuses to build without it." >&2
    exit 1
fi

# Any tag-shaped token in these files is a reference to a release.
PATTERN='v[0-9]+\.[0-9]+\.[0-9]+-hz[0-9]*'
FILES=(
    README.md
    README_zh.md
    deploy/update.sh
    deploy/docker/README.md
    deploy/docker/Dockerfile.npc
    deploy/docker/Dockerfile.nps
)

for f in "${FILES[@]}"; do
    [[ -f $f ]] || continue
    grep -qE "$PATTERN" "$f" || continue
    sed -i -E "s|$PATTERN|$TAG|g" "$f"
done

# The release pipeline stamps VERSION from the tag, so this default is what a
# build from source reports. Keeping it in step means such a build is not
# mistaken for some other release.
sed -i -E "s|^var VERSION = \".*\"|var VERSION = \"${TAG}\"|" lib/version/version.go

echo "Updated to ${TAG}:"
git --no-pager diff --stat
echo
echo "Any reference left behind:"
grep -rnE "$PATTERN" --include='*.md' --include='*.sh' --include='Dockerfile*' . \
    | grep -v "$TAG" \
    | grep -v '^./nps-mux/' \
    | grep -v '^./deploy/bump-version.sh' `# its own usage examples` \
    || echo "  none"
