#!/usr/bin/env bash
# Updates the tested upstream release (tag + SHA-256) in internal/install/upstream.go.
#   ./scripts/update-upstream-pin.sh            # newest upstream tag
#   ./scripts/update-upstream-pin.sh v1.4.61    # a specific tag
# Run it on your own computer, review the diff, run the tests, commit.
set -euo pipefail
cd "$(dirname "$0")/.."
REPO="Red-Blink/dune-awakening-selfhost-docker"
TAG="${1:-$(git ls-remote --tags "https://github.com/$REPO" | grep -v '\^{}' | sed 's#.*refs/tags/##' | sort -V | tail -1)}"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
curl -fL --proto '=https' -o "$TMP/a.tgz" "https://github.com/$REPO/archive/refs/tags/$TAG.tar.gz"
SUM="$(sha256sum "$TMP/a.tgz" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP/a.tgz" | awk '{print $1}')"
GOT="$(tar -xzOf "$TMP/a.tgz" --wildcards '*/VERSION' | tr -d '[:space:]')"
[ "$GOT" = "$TAG" ] || { echo "VERSION inside the archive is '$GOT', expected '$TAG'" >&2; exit 1; }
sed -i.bak -E "s|(UpstreamTag +=).*|\1 \"$TAG\"|; s|(UpstreamSHA256 =).*|\1 \"$SUM\"|" internal/install/upstream.go && rm -f internal/install/upstream.go.bak
echo "Pinned $TAG ($SUM). Review 'git diff', run 'go test ./...', then commit."
