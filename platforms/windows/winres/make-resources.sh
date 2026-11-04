#!/usr/bin/env bash
# Generates the Windows resource objects (exe icon, version info, manifest) as
# cmd/dd-installer/rsrc_windows_*.syso so the .exe carries the logo.
# Runs on the Windows release runner; the .syso files are git-ignored.
set -euo pipefail
cd "$(dirname "$0")/../../.."
V="$(tr -d '[:space:]' < VERSION)"
NUM="${V%%-*}"                 # 0.0.3-alpha -> 0.0.3
V4="${NUM}.0"                  # 0.0.3.0
OUT="platforms/windows/winres"
sed -e "s/@VERSION4@/$V4/g" -e "s/@VERSION@/$V/g" "$OUT/winres.json.in" > "$OUT/winres.json"
go run github.com/tc-hib/go-winres@v0.3.3 make --in "$OUT/winres.json" --out cmd/dd-installer/rsrc
ls -l cmd/dd-installer/*.syso
