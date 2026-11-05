#!/usr/bin/env bash
# Prints the release notes: the CHANGELOG.md section of VERSION plus a standard footer.
set -euo pipefail
cd "$(dirname "$0")/.."
V="$(tr -d '[:space:]' < VERSION)"
SECTION="$(awk -v v="$V" '$0 == "## " v {f=1; next} /^## / {f=0} f' CHANGELOG.md)"
[ -n "$SECTION" ] || SECTION="_No changelog entry for $V yet._"
cat <<NOTES
## DD-Installer $V

$SECTION

---
**Alpha.** Tested on real servers and desktop systems, but problems can still occur: make backups. The programs are not code-signed (Windows SmartScreen and macOS Gatekeeper will warn). Read the disclaimer in the archive: no credentials are stored and neither RedBlink nor Prophet can be held liable.
Verify downloads with \`SHA256SUMS\` (and the build provenance attestation of this repository). Built from commit ${GITHUB_SHA:-unknown}.
NOTES
