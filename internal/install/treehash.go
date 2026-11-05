package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// The release check does not hash the downloaded archive (its bytes depend on
// how GitHub compresses it and can change without any change in content) but
// the extracted FILES: one SHA-256 over the sorted list of "<sha256>  ./<path>"
// lines of every regular file. The same value is computed on the target by
// TreeHashShell and by TreeHash (used by cmd/pin-upstream and the tests).

// TreeHashShell defines dd_tree_hash <dir> for POSIX sh. Links are rejected
// before it is used, so every file of the release is covered.
const TreeHashShell = `dd_sha256() { if command -v sha256sum >/dev/null 2>&1; then sha256sum | cut -d' ' -f1; else shasum -a 256 | cut -d' ' -f1; fi; }
dd_tree_hash() {
  ( cd "$1" && find . -type f | LC_ALL=C sort | while IFS= read -r f; do
      printf '%s  %s\n' "$(dd_sha256 < "$f")" "$f"
    done | dd_sha256 )
}
`

// TreeHash computes the same value as TreeHashShell for a directory.
func TreeHash(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			paths = append(paths, "./"+filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths) // byte order, like LC_ALL=C sort
	total := sha256.New()
	for _, p := range paths {
		f, err := os.Open(filepath.Join(root, filepath.FromSlash(p[2:])))
		if err != nil {
			return "", err
		}
		h := sha256.New()
		_, cerr := io.Copy(h, f)
		f.Close()
		if cerr != nil {
			return "", cerr
		}
		fmt.Fprintf(total, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), p)
	}
	return hex.EncodeToString(total.Sum(nil)), nil
}
