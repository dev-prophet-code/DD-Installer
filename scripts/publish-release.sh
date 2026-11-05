#!/usr/bin/env bash
# Creates or updates the release v<VERSION> from ./dist and ./notes.md.
# Re-pushing the same version updates it in place (tag moved to this commit);
# assets of an OS that was not built in this run are removed so the release
# never mixes files from different commits.
set -euo pipefail
cd "$(dirname "$0")/.."
V="$(tr -d '[:space:]' < VERSION)"
TAG="v$V"
PRE=()
case "$V" in *-*) PRE=(--prerelease) ;; esac

git tag -f "$TAG" "${GITHUB_SHA:-HEAD}"
git push -f origin "refs/tags/$TAG"

EXPECTED=(dd-installer-linux-amd64.tar.gz dd-installer-windows-amd64.zip dd-installer-macos-arm64.tar.gz dd-installer-macos-amd64.tar.gz)
MISSING=()
for f in "${EXPECTED[@]}"; do [ -f "dist/$f" ] || MISSING+=("$f"); done
if [ ${#MISSING[@]} -gt 0 ]; then
  {
    echo
    echo "> **Not in this build:** ${MISSING[*]} (that runner did not finish). Re-run the **Release macOS** workflow to add the macOS build."
  } >> notes.md
fi

if gh release view "$TAG" >/dev/null 2>&1; then
  gh release edit "$TAG" --title "DD-Installer $V" --notes-file notes.md "${PRE[@]}"
  for f in "${MISSING[@]}"; do gh release delete-asset "$TAG" "$f" --yes 2>/dev/null || true; done
  gh release upload "$TAG" dist/* --clobber
else
  gh release create "$TAG" dist/* --title "DD-Installer $V" --notes-file notes.md --target "${GITHUB_SHA:-main}" "${PRE[@]}"
fi
echo "Published $TAG"
