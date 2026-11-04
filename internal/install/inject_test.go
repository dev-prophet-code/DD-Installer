package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// Pentest: hostile values must never execute as shell on the target.
func TestNoShellInjection(t *testing.T) {
	home := t.TempDir()
	pwned := filepath.Join(home, "PWNED")
	evil := []string{
		"x'; touch " + pwned + "; '", "$(touch " + pwned + ")", "`touch " + pwned + "`",
		"a\nDD_EOF\ntouch " + pwned + "\n", "\"; touch " + pwned + "; \"",
	}
	for _, v := range evil {
		cfg := model.Default()
		cfg.ServerTitle = "t"
		cfg.FuncomToken = v
		cfg.Sietches = []model.Sietch{{Name: "ok", Mode: model.PvE}}
		// tokens/titles/names travel as JSON inside a quoted heredoc
		scripts := []string{
			consoleScript(cfg, "setup", mustJSON(map[string]any{"port": 1, "passwordFile": "/nonexistent", "title": v, "region": "Europe", "token": v, "sietches": []string{v}})),
			tuningScript(cfg, map[string]string{"A": v}),
			sudoGrantScript("u"),
		}
		for _, s := range scripts {
			if strings.Contains(s, "DD_SUDO") { // never executed here (needs sudo)
				continue
			}
			// Do not start the real Python helper (it would wait for a console);
			// everything before it - the quoting/heredoc layer - is what is tested.
			s = strings.Join(filterLines(strings.Split(s, "\n"), `"$PY"`), "\n")
			var out strings.Builder
			r := &transport.Local{Cmd: transport.ShellArgs}
			// run in an isolated HOME with an install dir so `cd` succeeds
			os.MkdirAll(filepath.Join(home, "dune-awakening-selfhost-docker"), 0o755)
			_, _ = r.Run(context.Background(), "HOME="+home+"\nexport HOME\n"+s, &out)
			if _, err := os.Stat(pwned); err == nil {
				t.Fatalf("INJECTION with value %q", v)
			}
		}
	}
	// Config validation also blocks hostile paths/names before any script exists.
	for _, v := range []string{"/a;touch x", "/a$(id)", "/a`id`", "/a b\"c", "~/a|b"} {
		c := model.Default()
		c.InstallDir = v
		if len(c.Validate()) == 0 {
			t.Errorf("installDir %q accepted", v)
		}
	}
}

func filterLines(in []string, drop string) []string {
	var out []string
	for _, l := range in {
		if !strings.Contains(l, drop) {
			out = append(out, l)
		}
	}
	return out
}
