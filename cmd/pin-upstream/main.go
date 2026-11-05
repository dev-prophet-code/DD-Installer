// Command pin-upstream records an upstream Dune Docker release (tag and the
// SHA-256 of its files) in upstream/upstream.json, the list DD-Installer reads
// before every download.
//
//	go run ./cmd/pin-upstream -latest             # newest release -> "latest" (also run daily by CI)
//	go run ./cmd/pin-upstream -tag v1.4.61 -tested # a specific release -> "tested" + built-in pin
//	go run ./cmd/pin-upstream -tag v1.4.61 -tested -dir ./extracted   # offline: hash an existing folder
//
// "tested" is only ever changed by hand, after the release was verified with
// DD-Installer. Review the diff, run the tests, commit.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/install"
)

func main() {
	tag := flag.String("tag", "", "release tag such as v1.4.61")
	latest := flag.Bool("latest", false, "look up the newest release")
	tested := flag.Bool("tested", false, "record as the tested release (default: record as latest)")
	dir := flag.String("dir", "", "hash this already extracted folder instead of downloading")
	manifest := flag.String("manifest", "upstream/upstream.json", "manifest file")
	builtin := flag.String("builtin", "internal/install/upstream.go", "source file with the built-in pin (used with -tested)")
	flag.Parse()
	if err := run(*tag, *latest, *tested, *dir, *manifest, *builtin); err != nil {
		fmt.Fprintln(os.Stderr, "pin-upstream:", err)
		os.Exit(1)
	}
}

func run(tag string, latest, tested bool, dir, manifestPath, builtinPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if latest {
		t, err := newestTag(ctx)
		if err != nil {
			return err
		}
		tag = t
	}
	pin := install.Pin{Tag: tag}
	if err := (install.Pin{Tag: tag, TreeSHA256: strings.Repeat("0", 64)}).Valid(); err != nil {
		return err
	}
	root := dir
	if root == "" {
		tmp, err := os.MkdirTemp("", "pin-upstream-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := download(ctx, tag, tmp); err != nil {
			return err
		}
		root = tmp
	}
	if v, err := os.ReadFile(filepath.Join(root, "VERSION")); err != nil || strings.TrimSpace(string(v)) != tag {
		return fmt.Errorf("the VERSION file inside the release is %q, expected %q", strings.TrimSpace(string(v)), tag)
	}
	sum, err := install.TreeHash(root)
	if err != nil {
		return err
	}
	pin.TreeSHA256 = sum

	m := install.Manifest{Schema: 1}
	if b, err := os.ReadFile(manifestPath); err == nil {
		if m, err = install.ParseManifest(b); err != nil {
			return err
		}
	}
	if tested {
		m.Tested = pin
		if m.Latest.Tag == "" {
			m.Latest = pin
		}
		if err := rewriteBuiltin(builtinPath, pin); err != nil {
			return err
		}
	} else {
		if m.Tested.Tag == "" {
			return fmt.Errorf("the manifest has no tested release yet; run with -tested first")
		}
		if m.Latest == pin {
			fmt.Println("up to date:", tag, sum)
			return nil
		}
		m.Latest = pin
	}
	m.Updated = time.Now().UTC().Format("2006-01-02")
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, append(out, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("recorded %s (%s)\n", tag, sum)
	return nil
}

var builtinRe = regexp.MustCompile(`(?m)^(\s*UpstreamTag\s*=\s*)"[^"]*"|^(\s*UpstreamTreeSHA256\s*=\s*)"[^"]*"`)

func rewriteBuiltin(file string, p install.Pin) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	n := 0
	out := builtinRe.ReplaceAllStringFunc(string(b), func(m string) string {
		n++
		if strings.Contains(m, "UpstreamTreeSHA256") {
			return m[:strings.Index(m, `"`)] + `"` + p.TreeSHA256 + `"`
		}
		return m[:strings.Index(m, `"`)] + `"` + p.Tag + `"`
	})
	if n != 2 {
		return fmt.Errorf("%s: expected to find UpstreamTag and UpstreamTreeSHA256, found %d of them", file, n)
	}
	return os.WriteFile(file, []byte(out), 0o644)
}

var semverTag = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

// newestTag returns the highest plain vX.Y.Z tag of the upstream repository.
func newestTag(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+install.UpstreamRepo+"/tags?per_page=100", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("listing tags: HTTP %d", resp.StatusCode)
	}
	var tags []struct{ Name string }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&tags); err != nil {
		return "", err
	}
	var names []string
	for _, t := range tags {
		if semverTag.MatchString(t.Name) {
			names = append(names, t.Name)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no vX.Y.Z tags found")
	}
	key := func(s string) [3]int {
		m := semverTag.FindStringSubmatch(s)
		var k [3]int
		for i := 0; i < 3; i++ {
			fmt.Sscan(m[i+1], &k[i])
		}
		return k
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := key(names[i]), key(names[j])
		for k := 0; k < 3; k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})
	return names[len(names)-1], nil
}

// download fetches the tag archive and extracts it (first path element
// stripped) into dst. Absolute paths, "..", links and special files are refused.
func download(ctx context.Context, tag, dst string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/"+install.UpstreamRepo+"/archive/refs/tags/"+tag+".tar.gz", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("downloading %s: HTTP %d", tag, resp.StatusCode)
	}
	gz, err := gzip.NewReader(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := path.Clean(h.Name)
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			if h.Typeflag == tar.TypeDir {
				continue
			}
			return fmt.Errorf("unsafe archive entry %q", h.Name)
		}
		target := filepath.Join(dst, filepath.FromSlash(parts[1]))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("the archive contains a link or special file: %q", h.Name)
		}
	}
}
