#!/usr/bin/env bash
# Adds the archives in ./dist to the existing release v<VERSION> and rebuilds SHA256SUMS.
set -euo pipefail
cd "$(dirname "$0")/.."
V="$(tr -d '[:space:]' < VERSION)"
TAG="v$V"
gh release view "$TAG" >/dev/null 2>&1 || { echo "Release $TAG does not exist yet - run the Release workflow first." >&2; exit 1; }
rm -rf all && mkdir all
gh release download "$TAG" --dir all --pattern '*.tar.gz' --pattern '*.zip' || true
cp dist/*.tar.gz dist/*.zip all/ 2>/dev/null || true
(cd all && shasum -a 256 *.tar.gz *.zip > SHA256SUMS)
gh release upload "$TAG" dist/*.tar.gz all/SHA256SUMS --clobber
echo "Added the macOS build to $TAG"
