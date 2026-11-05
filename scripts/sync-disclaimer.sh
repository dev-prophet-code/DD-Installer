#!/usr/bin/env bash
# docs/DISCLAIMER.md is a copy of the single source internal/app/disclaimer.md.
set -euo pipefail
cd "$(dirname "$0")/.."
cp internal/app/disclaimer.md docs/DISCLAIMER.md
echo "docs/DISCLAIMER.md updated"
