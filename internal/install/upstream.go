package install

import (
	"fmt"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// Upstream pin. DD-Installer is tested against this release of
// Red-Blink/dune-awakening-selfhost-docker. By default exactly this version is
// installed (not "whatever is newest"), because the Console API and CLI that
// the installer drives can change between upstream releases.
//
// Update with scripts/update-upstream-pin.sh (it downloads the tag archive and
// writes the new tag and SHA-256 here).
const (
	UpstreamRepo = "Red-Blink/dune-awakening-selfhost-docker"
	UpstreamTag  = "v1.4.61"
	// UpstreamSHA256 is the SHA-256 of the GitHub tag archive. When empty the
	// archive is not hash-checked (the tag is still pinned and the VERSION
	// file inside is verified).
	UpstreamSHA256 = ""
)

// pinnedBootstrapScript downloads the pinned upstream release with the same
// safety checks as upstream's own bootstrap.sh (no overwrite of an existing
// folder, free-space and write test, hash check, unsafe-path check, staged
// extraction) and leaves the folder ready for ./install.sh.
func pinnedBootstrapScript(cfg model.Config) string {
	var b strings.Builder
	b.WriteString(withPrelude("set -e\n"))
	fmt.Fprintf(&b, "DIR=\"%s\"\nTAG=%s\nEXPECTED=%s\n", dirExpr(cfg), shq(UpstreamTag), shq(UpstreamSHA256))
	fmt.Fprintf(&b, "URL=%s\n", shq("https://github.com/"+UpstreamRepo+"/archive/refs/tags/"+UpstreamTag+".tar.gz"))
	b.WriteString(`fail() { echo "Installation could not continue: $1" >&2; exit 1; }
if [ -f "$DIR/install.sh" ]; then
  echo "Existing Dune Docker found in $DIR - reusing it (nothing is overwritten)."
  exit 0
fi
[ -e "$DIR" ] && fail "$DIR already exists. Move or remove that incomplete directory, or choose another install folder."
command -v tar >/dev/null 2>&1 || fail "tar is missing. Install curl and tar with your distribution's package manager, then retry."
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fL --proto '=https' --retry 3 --connect-timeout 20 "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -O "$2" "$1"; }
else
  fail "curl or wget is required. Install curl and tar with your distribution's package manager, then retry."
fi
PARENT=$(dirname "$DIR")
mkdir -p "$PARENT" 2>/dev/null || fail "cannot create $PARENT. Choose a writable Linux directory."
AVAILABLE_KB=$(df -Pk "$PARENT" 2>/dev/null | awk 'NR == 2 { print $4 }')
case "$AVAILABLE_KB" in ''|*[!0-9]*) ;; *) [ "$AVAILABLE_KB" -ge 102400 ] || fail "$PARENT has less than 100 MB free before Docker images are downloaded. Choose a larger Linux filesystem." ;; esac
TMP=$(mktemp -d "$PARENT/.dd-install.XXXXXX") || fail "cannot create a temporary directory in $PARENT."
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
echo "==> Downloading Dune Docker $TAG (tested version)..."
fetch "$URL" "$TMP/release.tar.gz" || fail "the release download failed. Check free space and internet access, then retry."
if [ -n "$EXPECTED" ]; then
  if command -v sha256sum >/dev/null 2>&1; then ACTUAL=$(sha256sum "$TMP/release.tar.gz" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then ACTUAL=$(shasum -a 256 "$TMP/release.tar.gz" | awk '{print $1}')
  else fail "sha256sum or shasum is required to verify the release archive."; fi
  [ "$ACTUAL" = "$EXPECTED" ] || fail "release archive SHA-256 mismatch (expected $EXPECTED, got $ACTUAL)."
  echo "Archive checksum verified."
fi
if tar -tzf "$TMP/release.tar.gz" | grep -Eq '(^/|(^|/)\.\.(/|$))'; then fail "the release archive contains unsafe path entries."; fi
mkdir "$TMP/x"
tar -xzf "$TMP/release.tar.gz" -C "$TMP/x" --strip-components=1 || fail "the release archive could not be extracted. Check free space and filesystem permissions."
[ -f "$TMP/x/install.sh" ] || fail "the downloaded release is incomplete."
GOT=$(tr -d '[:space:]' < "$TMP/x/VERSION" 2>/dev/null || true)
[ "$GOT" = "$TAG" ] || fail "the archive contains version '$GOT' instead of $TAG."
mv "$TMP/x" "$DIR" || fail "the prepared installation could not be moved into $DIR."
chmod +x "$DIR/install.sh"
echo "==> Installed release files in $DIR"
`)
	return b.String()
}

// tested reports whether the installed upstream version is the one DD-Installer
// was tested with.
func tested(version string) bool { return strings.TrimSpace(version) == UpstreamTag }
