package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// Upstream release handling.
//
// DD-Installer installs a release of Red-Blink/dune-awakening-selfhost-docker
// and checks the downloaded files against a SHA-256 before anything is run.
// RedBlink publishes new releases regularly, so the expected value is not only
// compiled in: before every download the installer reads the current manifest
// (upstream/upstream.json in this repository, updated by cmd/pin-upstream and
// the "Upstream manifest" workflow). A new Dune Docker release therefore needs
// a new manifest entry, not a new DD-Installer build.
//
//   - "tested" is the release DD-Installer was verified against (default).
//   - "latest" is the newest upstream release (opt-in, may be incompatible).
//
// If the manifest cannot be read, the built-in tested pin below is used, so an
// install works offline from GitHub's raw host too. Without a checksum nothing
// is installed.
const (
	UpstreamRepo = "Red-Blink/dune-awakening-selfhost-docker"
	// Built-in fallback pin (kept in sync with upstream/upstream.json "tested"
	// by cmd/pin-upstream and a test).
	UpstreamTag = "v1.4.61"
	// UpstreamTreeSHA256 is TreeHash of the extracted release files.
	UpstreamTreeSHA256 = "c5410cff1f2df667d0fdd1af321c9f6d1a1b429d8221af236de054e594a52811"

	// ManifestURL is where the current expected hashes are read from.
	ManifestURL = "https://raw.githubusercontent.com/dev-prophet-code/DD-Installer/main/upstream/upstream.json"
)

// Pin names one upstream release and the expected hash of its files.
type Pin struct {
	Tag        string `json:"tag"`
	TreeSHA256 string `json:"tree_sha256"`
}

// Manifest is the content of upstream/upstream.json.
type Manifest struct {
	Schema  int    `json:"schema"`
	Updated string `json:"updated,omitempty"`
	Tested  Pin    `json:"tested"`
	Latest  Pin    `json:"latest"`
}

var (
	tagRe  = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Valid reports whether the pin is well-formed (the tag and hash end up in a
// shell script and a URL, so nothing else is accepted).
func (p Pin) Valid() error {
	if !tagRe.MatchString(p.Tag) {
		return fmt.Errorf("invalid release tag %q", p.Tag)
	}
	if !hashRe.MatchString(p.TreeSHA256) {
		return fmt.Errorf("invalid checksum for %s", p.Tag)
	}
	return nil
}

// BuiltinPin is the pin compiled into this version.
func BuiltinPin() Pin { return Pin{Tag: UpstreamTag, TreeSHA256: UpstreamTreeSHA256} }

// ParseManifest reads and validates a manifest. The tested pin is mandatory.
func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(b)))
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: %w", err)
	}
	if m.Schema != 1 {
		return Manifest{}, fmt.Errorf("manifest: unsupported schema %d", m.Schema)
	}
	if err := m.Tested.Valid(); err != nil {
		return Manifest{}, fmt.Errorf("manifest tested: %w", err)
	}
	if m.Latest.Tag != "" || m.Latest.TreeSHA256 != "" {
		if err := m.Latest.Valid(); err != nil {
			return Manifest{}, fmt.Errorf("manifest latest: %w", err)
		}
	}
	return m, nil
}

// FetchManifest downloads the current manifest over HTTPS (small, time-boxed).
func FetchManifest(ctx context.Context) (Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ManifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("manifest: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return Manifest{}, err
	}
	return ParseManifest(b)
}

// ResolvePin decides which release to install and which hash to expect. The
// manifest is read first; the built-in pin is the fallback. It also returns the
// tag DD-Installer regards as "tested" and a message for the log.
func ResolvePin(ctx context.Context, latest bool, fetch func(context.Context) (Manifest, error)) (pin Pin, tested string, msg string, err error) {
	builtin := BuiltinPin()
	if fetch == nil {
		fetch = FetchManifest
	}
	m, ferr := fetch(ctx)
	if ferr != nil {
		if verr := builtin.Valid(); verr != nil {
			return Pin{}, "", "", errors.New("no checksum is available for the Dune Docker download: the current list could not be read (" + ferr.Error() + ") and this build has none built in. Check your internet access and try again")
		}
		msg = "WARNING: could not read the current checksum list (" + ferr.Error() + "); using the built-in " + builtin.Tag + "."
		if latest {
			msg += " The newest release cannot be verified without the list, so the tested one is installed."
		}
		return builtin, builtin.Tag, msg, nil
	}
	if latest && m.Latest.Valid() == nil {
		return m.Latest, m.Tested.Tag, "Checksum list read: installing the newest release " + m.Latest.Tag + " (verified by SHA-256 before use).", nil
	}
	if latest {
		msg = "The newest release is not listed yet; installing the tested " + m.Tested.Tag + " instead. "
	}
	return m.Tested, m.Tested.Tag, msg + "Checksum list read: installing " + m.Tested.Tag + " (verified by SHA-256 before use).", nil
}

// pinnedBootstrapScript downloads the chosen upstream release, verifies the
// SHA-256 of all its files and only then moves it into place, with the same
// safety checks as upstream's own bootstrap.sh (no overwrite of an existing
// folder, free-space and write test, unsafe-path and link check, staged
// extraction). Nothing from the download is executed before the check passed.
func pinnedBootstrapScript(cfg model.Config, pin Pin) string {
	var b strings.Builder
	b.WriteString(withPrelude("set -e\n"))
	fmt.Fprintf(&b, "DIR=\"%s\"\nTAG=%s\nEXPECTED=%s\n", dirExpr(cfg), shq(pin.Tag), shq(pin.TreeSHA256))
	fmt.Fprintf(&b, "URL=%s\n", shq("https://github.com/"+UpstreamRepo+"/archive/refs/tags/"+pin.Tag+".tar.gz"))
	b.WriteString(TreeHashShell)
	b.WriteString(`fail() { echo "Installation could not continue: $1" >&2; exit 1; }
[ -n "$EXPECTED" ] || fail "no checksum is known for this release - refusing to install unverified files."
if [ -f "$DIR/install.sh" ]; then
  echo "Existing Dune Docker found in $DIR - reusing it (nothing is overwritten)."
  exit 0
fi
[ -e "$DIR" ] && fail "$DIR already exists. Move or remove that incomplete directory, or choose another install folder."
command -v tar >/dev/null 2>&1 || fail "tar is missing. Install curl and tar with your distribution's package manager, then retry."
command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || fail "sha256sum or shasum is required to verify the release files."
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
echo "==> Downloading Dune Docker $TAG..."
fetch "$URL" "$TMP/release.tar.gz" || fail "the release download failed. Check free space and internet access, then retry."
if tar -tzf "$TMP/release.tar.gz" | grep -Eq '(^/|(^|/)\.\.(/|$))'; then fail "the release archive contains unsafe path entries."; fi
if tar -tvzf "$TMP/release.tar.gz" | grep -Eq '^[lh]'; then fail "the release archive contains links, which are not allowed."; fi
mkdir "$TMP/x"
tar -xzf "$TMP/release.tar.gz" -C "$TMP/x" --strip-components=1 || fail "the release archive could not be extracted. Check free space and filesystem permissions."
ACTUAL=$(dd_tree_hash "$TMP/x")
[ "$ACTUAL" = "$EXPECTED" ] || fail "the downloaded files do not match the expected SHA-256 (expected $EXPECTED, got $ACTUAL). Nothing was installed."
echo "Checksum verified: the downloaded files are exactly the expected release."
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
func tested(version, testedTag string) bool { return strings.TrimSpace(version) == testedTag }
