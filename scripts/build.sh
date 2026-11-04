#!/usr/bin/env bash
# Cross-compiles dd-installer for every supported OS/arch into dist/.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-0.1.0-draft}"
rm -rf dist && mkdir -p dist
build() { # os arch outname
  GOOS=$1 GOARCH=$2 CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/dev-prophet-code/dd-installer/internal/gui.Version=$VERSION" \
    -o "dist/$3" ./cmd/dd-installer
}
build linux   amd64 dd-installer-linux-amd64
build linux   arm64 dd-installer-linux-arm64
build darwin  amd64 dd-installer-macos-amd64
build darwin  arm64 dd-installer-macos-arm64
build windows amd64 dd-installer-windows-amd64.exe
(cd dist && sha256sum * > SHA256SUMS)
ls -l dist
