#!/usr/bin/env bash
# Records an upstream Dune Docker release in upstream/upstream.json (and, with
# --tested, in the built-in fallback in internal/install/upstream.go).
#   ./scripts/update-upstream-pin.sh                  # newest release -> "latest"
#   ./scripts/update-upstream-pin.sh v1.4.61 --tested # a verified release -> "tested"
# The download, the SHA-256 of all files and the VERSION check are done by
# cmd/pin-upstream. Review the diff, run the tests, commit.
set -euo pipefail
cd "$(dirname "$0")/.."
TAG="${1:-}"
case "$TAG" in
  "")  exec go run ./cmd/pin-upstream -latest ;;
  v*)  shift; exec go run ./cmd/pin-upstream -tag "$TAG" "$([ "${1:-}" = "--tested" ] && echo -tested || echo -latest=false)" ;;
  *)   echo "usage: $0 [vX.Y.Z [--tested]]" >&2; exit 2 ;;
esac
