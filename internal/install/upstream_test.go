package install

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

func mkTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The hash computed by the generated shell script on the server and the one
// computed by Go (cmd/pin-upstream) must be identical, including for awkward
// file names and sort orders.
func TestTreeHashShellMatchesGo(t *testing.T) {
	unixOnly(t)
	root := mkTree(t, map[string]string{
		"VERSION": "v1.2.3\n", "install.sh": "#!/bin/sh\n", "B": "upper", "b": "lower", "a b": "space",
		"a/x": "dir", "a-b": "dash", "a.b": "dot", "Ä/ü.txt": "unicode", "deep/er/est/f": "x", "empty": "",
	})
	want, err := TreeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runSh(t, TreeHashShell+"dd_tree_hash '"+root+"'\n")
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := strings.TrimSpace(out); got != want {
		t.Fatalf("shell hash %s != Go hash %s", got, want)
	}
	// any change in content, name or an extra file changes the hash
	for name, mut := range map[string]func(){
		"content": func() { _ = os.WriteFile(filepath.Join(root, "b"), []byte("LOWER"), 0o644) },
		"extra":   func() { _ = os.WriteFile(filepath.Join(root, "evil.sh"), []byte("x"), 0o644) },
		"rename":  func() { _ = os.Rename(filepath.Join(root, "B"), filepath.Join(root, "C")) },
	} {
		before, _ := TreeHash(root)
		mut()
		after, _ := TreeHash(root)
		if before == after {
			t.Errorf("%s change was not detected", name)
		}
	}
}

// The compiled-in fallback must equal the "tested" entry of the manifest file
// that is committed next to it.
func TestBuiltinPinMatchesTheCommittedManifest(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "upstream", "upstream.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(b)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tested != BuiltinPin() {
		t.Fatalf("built-in %+v differs from manifest tested %+v; run: go run ./cmd/pin-upstream -tag %s -tested", BuiltinPin(), m.Tested, m.Tested.Tag)
	}
	if err := BuiltinPin().Valid(); err != nil {
		t.Fatal(err)
	}
}

func TestParseManifestRejectsMalformedData(t *testing.T) {
	h := strings.Repeat("a", 64)
	good := `{"schema":1,"tested":{"tag":"v1.2.3","tree_sha256":"` + h + `"},"latest":{"tag":"v1.2.4","tree_sha256":"` + h + `"}}`
	if _, err := ParseManifest([]byte(good)); err != nil {
		t.Fatal(err)
	}
	bad := []string{
		``, `{`, `[]`,
		`{"schema":2,"tested":{"tag":"v1.2.3","tree_sha256":"` + h + `"}}`,
		`{"schema":1,"tested":{"tag":"v1.2.3","tree_sha256":"short"}}`,
		`{"schema":1,"tested":{"tag":"v1.2.3'; id; '","tree_sha256":"` + h + `"}}`,
		`{"schema":1,"tested":{"tag":"main","tree_sha256":"` + h + `"}}`,
		`{"schema":1,"tested":{"tag":"v1.2.3","tree_sha256":"` + strings.ToUpper(h) + `"}}`,
		`{"schema":1,"tested":{"tag":"v1.2.3","tree_sha256":"` + h + `"},"latest":{"tag":"v9","tree_sha256":"` + h + `"}}`,
		`{"schema":1}`,
	}
	for _, b := range bad {
		if _, err := ParseManifest([]byte(b)); err == nil {
			t.Errorf("accepted %q", b)
		}
	}
}

func TestResolvePin(t *testing.T) {
	h1, h2 := strings.Repeat("1", 64), strings.Repeat("2", 64)
	m := Manifest{Schema: 1, Tested: Pin{"v1.0.0", h1}, Latest: Pin{"v1.1.0", h2}}
	ok := func(context.Context) (Manifest, error) { return m, nil }

	pin, tested, _, err := ResolvePin(context.Background(), false, ok)
	if err != nil || pin != m.Tested || tested != "v1.0.0" {
		t.Fatalf("tested: %+v %q %v", pin, tested, err)
	}
	pin, tested, msg, err := ResolvePin(context.Background(), true, ok)
	if err != nil || pin != m.Latest || tested != "v1.0.0" || !strings.Contains(msg, "v1.1.0") {
		t.Fatalf("latest: %+v %q %q %v", pin, tested, msg, err)
	}
	// latest not listed -> tested release, and the user is told
	m2 := m
	m2.Latest = Pin{}
	pin, _, msg, err = ResolvePin(context.Background(), true, func(context.Context) (Manifest, error) { return m2, nil })
	if err != nil || pin != m.Tested || !strings.Contains(msg, "not listed") {
		t.Fatalf("latest missing: %+v %q %v", pin, msg, err)
	}
	// the list cannot be read -> built-in pin, with a warning; "latest" is NOT guessed
	down := func(context.Context) (Manifest, error) { return Manifest{}, errors.New("offline") }
	for _, latest := range []bool{false, true} {
		pin, _, msg, err = ResolvePin(context.Background(), latest, down)
		if err != nil || pin != BuiltinPin() || !strings.Contains(msg, "WARNING") {
			t.Fatalf("fallback (latest=%v): %+v %q %v", latest, pin, msg, err)
		}
	}
}

// The release script must verify the checksum BEFORE anything of the download
// is moved into place or executed, and must refuse to run without a checksum.
func TestBootstrapScriptVerifiesBeforeInstalling(t *testing.T) {
	cfg := model.Default()
	s := pinnedBootstrapScript(cfg, BuiltinPin())
	iHash := strings.Index(s, "dd_tree_hash \"$TMP/x\"")
	iCmp := strings.Index(s, `[ "$ACTUAL" = "$EXPECTED" ]`)
	iMove := strings.Index(s, `mv "$TMP/x"`)
	if iHash < 0 || iCmp < iHash || iMove < iCmp {
		t.Fatalf("order must be: hash, compare, move (%d %d %d)", iHash, iCmp, iMove)
	}
	if !strings.Contains(s, "refusing to install unverified files") {
		t.Fatal("an empty checksum must abort the installation")
	}
	if strings.Contains(s, "| sh") || strings.Contains(s, "bash -s") {
		t.Fatal("no piping of downloaded text into a shell")
	}
}

// End to end with a real tarball and a real shell: the correct hash installs,
// a manipulated archive, a missing hash and a link in the archive are refused.
func TestBootstrapScriptEndToEnd(t *testing.T) {
	unixOnly(t)
	for _, tool := range []string{"curl", "tar", "sha256sum", "df", "mktemp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " is not available")
		}
	}
	release := map[string]string{"VERSION": "v1.2.3\n", "install.sh": "#!/bin/sh\necho hi\n", "docker-compose.yml": "x\n", "runtime/scripts/dune": "#!/bin/sh\n"}
	pack := func(files map[string]string, extra ...string) string {
		src := mkTree(t, files)
		parent := filepath.Join(t.TempDir(), "dune-awakening-selfhost-docker-1.2.3")
		if err := os.Rename(src, parent); err != nil {
			t.Fatal(err)
		}
		tgz := filepath.Join(t.TempDir(), "release.tar.gz")
		args := append([]string{"-czf", tgz, "-C", filepath.Dir(parent), filepath.Base(parent)}, extra...)
		if out, err := exec.Command("tar", args...).CombinedOutput(); err != nil {
			t.Fatalf("tar: %v %s", err, out)
		}
		return tgz
	}
	good := pack(release)
	// hash of the files as the server will see them after extraction
	ref := mkTree(t, release)
	want, _ := TreeHash(ref)

	run := func(tgz, expected string) (string, string, error) {
		home := t.TempDir()
		pin := Pin{Tag: "v1.2.3", TreeSHA256: expected}
		s := pinnedBootstrapScript(model.Default(), pin)
		// offline test: fetch the local file instead of github.com
		s = strings.Replace(s, "--proto '=https'", "--proto '=file'", 1)
		s = regexp.MustCompile(`(?m)^URL=.*$`).ReplaceAllString(s, "URL='file://"+tgz+"'")
		out, err := runSh(t, s, "HOME="+home)
		return out, home, err
	}

	out, home, err := run(good, want)
	if err != nil || !strings.Contains(out, "Checksum verified") {
		t.Fatalf("a correct release must install: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, "dune-awakening-selfhost-docker", "install.sh")); err != nil {
		t.Fatal("install.sh was not moved into place")
	}

	// manipulated content -> refused, nothing installed
	evil := pack(map[string]string{"VERSION": "v1.2.3\n", "install.sh": "#!/bin/sh\nrm -rf ~\n", "docker-compose.yml": "x\n", "runtime/scripts/dune": "#!/bin/sh\n"})
	out, home, err = run(evil, want)
	if err == nil || !strings.Contains(out, "do not match the expected SHA-256") {
		t.Fatalf("a manipulated release must be refused: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, "dune-awakening-selfhost-docker")); err == nil {
		t.Fatal("manipulated files were installed")
	}

	// no checksum at all -> refused
	out, _, err = run(good, "")
	if err == nil || !strings.Contains(out, "refusing to install unverified files") {
		t.Fatalf("an empty checksum must be refused: %v\n%s", err, out)
	}

	// a symlink inside the archive -> refused (links are not covered by the hash)
	src := mkTree(t, release)
	_ = os.Symlink("/etc/passwd", filepath.Join(src, "link"))
	parent := filepath.Join(t.TempDir(), "dune-awakening-selfhost-docker-1.2.3")
	_ = os.Rename(src, parent)
	linked := filepath.Join(t.TempDir(), "linked.tar.gz")
	if o, e := exec.Command("tar", "-czf", linked, "-C", filepath.Dir(parent), filepath.Base(parent)).CombinedOutput(); e != nil {
		t.Fatalf("tar: %v %s", e, o)
	}
	out, _, err = run(linked, want)
	if err == nil || !strings.Contains(out, "contains links") {
		t.Fatalf("a release with links must be refused: %v\n%s", err, out)
	}
}
