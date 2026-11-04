#!/usr/bin/env bash
# Builds the native installer for the OS this script runs on and packages it
# into dist/. The Fyne GUI needs CGO, so each OS is built on its own runner
# (see .github/workflows/release.yml); there is no cross-compilation.
#
#   Linux:   sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev
#   macOS:   Xcode command line tools
#   Windows: a MinGW-w64 gcc on PATH (preinstalled on GitHub runners)
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-0.2.0-draft}"
LDFLAGS="-s -w -X github.com/dev-prophet-code/dd-installer/internal/app.Version=$VERSION"
rm -rf dist && mkdir -p dist/stage

pack() { # name ext archive-kind
  local dir="dist/stage/$1"
  cp README.md docs/DISCLAIMER.md "$dir/"
  case "$3" in
    zip)
      if command -v zip >/dev/null 2>&1; then
        (cd dist/stage && zip -qr "../$1.zip" "$1")
      else # Git Bash on Windows has no zip; PowerShell is always there
        powershell.exe -NoProfile -Command "Compress-Archive -Path 'dist/stage/$1' -DestinationPath 'dist/$1.zip' -Force"
      fi ;;
    tgz) tar -C dist/stage -czf "dist/$1.tar.gz" "$1" ;;
  esac
}

case "$(uname -s)" in
  Linux)
    name=dd-installer-linux-amd64; mkdir -p "dist/stage/$name"
    CGO_ENABLED=1 go build -trimpath -ldflags "$LDFLAGS" -o "dist/stage/$name/dd-installer" ./cmd/dd-installer
    pack "$name" "" tgz ;;
  Darwin)
    for arch in arm64 amd64; do
      name="dd-installer-macos-$arch"; mkdir -p "dist/stage/$name"
      CGO_ENABLED=1 GOARCH=$arch go build -trimpath -ldflags "$LDFLAGS" -o "dist/stage/$name/dd-installer" ./cmd/dd-installer
      pack "$name" "" tgz
    done ;;
  MINGW*|MSYS*|CYGWIN*)
    name=dd-installer-windows-amd64; mkdir -p "dist/stage/$name"
    CGO_ENABLED=1 go build -trimpath -ldflags "$LDFLAGS -H=windowsgui" -o "dist/stage/$name/dd-installer.exe" ./cmd/dd-installer
    pack "$name" "" zip ;;
  *) echo "unsupported OS $(uname -s)" >&2; exit 1 ;;
esac
rm -rf dist/stage
ls -l dist
