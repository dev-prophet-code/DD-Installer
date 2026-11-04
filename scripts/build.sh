#!/usr/bin/env bash
# Cross-compiles dd-installer for every supported OS/arch into dist/ and
# packages each build (tar.gz keeps the executable bit; zip for Windows).
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-0.1.0-draft}"
rm -rf dist && mkdir -p dist
build() { # os arch name ext
  local bin="dd-installer$4" dir="dist/stage/$3"
  mkdir -p "$dir"
  GOOS=$1 GOARCH=$2 CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/dev-prophet-code/dd-installer/internal/gui.Version=$VERSION" \
    -o "$dir/$bin" ./cmd/dd-installer
  cp README.md docs/DISCLAIMER.md "$dir/"
  if [ "$1" = windows ]; then
    (cd dist/stage && zip -qr "../$3.zip" "$3")
  else
    tar -C dist/stage -czf "dist/$3.tar.gz" "$3"
  fi
}
build linux   amd64 dd-installer-linux-amd64   ""
build linux   arm64 dd-installer-linux-arm64   ""
build darwin  amd64 dd-installer-macos-amd64   ""
build darwin  arm64 dd-installer-macos-arm64   ""
build windows amd64 dd-installer-windows-amd64 ".exe"
rm -rf dist/stage
(cd dist && sha256sum * > SHA256SUMS)
ls -l dist
