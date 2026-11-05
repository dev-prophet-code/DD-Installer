package manage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

func run(t *testing.T, home, script string) (string, int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		// the scripts target Linux servers and use Unix paths; Windows host paths break the shell quoting
		t.Skip("shell-execution tests of Linux server scripts run on Unix hosts only")
	}
	var b strings.Builder
	code, err := (&transport.Local{Cmd: transport.ShellArgs}).Run(context.Background(), "HOME="+home+"\nexport HOME\n"+script, &b)
	if err != nil {
		t.Fatal(err)
	}
	return b.String(), code
}

func TestActionsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range Actions() {
		if a.ID == "" || a.Title == "" || a.Description == "" || a.Script == nil {
			t.Errorf("incomplete action %+v", a)
		}
		if seen[a.ID] {
			t.Errorf("duplicate id %s", a.ID)
		}
		seen[a.ID] = true
		if a.Risk == Destructive && a.Phrase == "" {
			t.Errorf("%s: destructive actions need a confirmation phrase", a.ID)
		}
		if a.Risk != Destructive && a.Phrase != "" {
			t.Errorf("%s: only destructive actions use a phrase", a.ID)
		}
		if s := a.Script(model.Default()); !strings.Contains(s, "set -u") {
			t.Errorf("%s: script lacks the shell prelude", a.ID)
		}
	}
	if _, ok := ByID("uninstall-all"); !ok {
		t.Fatal("missing uninstall action")
	}
}

func TestUninstallRefusesUnsafeFolders(t *testing.T) {
	home := t.TempDir()
	keep := filepath.Join(home, "precious.txt")
	os.WriteFile(keep, []byte("data"), 0o644)

	cases := map[string]string{
		"home itself":    "",           // empty dir -> $HOME/dune-awakening-selfhost-docker (missing)
		"no markers":     "somefolder", // exists but is not a Dune Docker folder
		"home via tilde": "~/",         // resolves to $HOME/
		"root":           "/",
		"system folder":  "/usr",
	}
	os.MkdirAll(filepath.Join(home, "somefolder"), 0o755)
	os.WriteFile(filepath.Join(home, "somefolder", "x"), []byte("x"), 0o644)
	for name, dir := range cases {
		cfg := model.Default()
		switch {
		case dir == "~/":
			cfg.InstallDir = "~/"
		case dir == "/" || dir == "/usr":
			cfg.InstallDir = dir
		case dir != "":
			cfg.InstallDir = "~/" + dir
		}
		out, code := run(t, home, uninstallAllScript(cfg))
		if code == 0 {
			t.Errorf("%s: must refuse, got success: %s", name, out)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("a guard failed: files in HOME were deleted")
	}
	if _, err := os.Stat(filepath.Join(home, "somefolder", "x")); err != nil {
		t.Fatal("a folder without Dune Docker markers was deleted")
	}
}

func TestUninstallRemovesOnlyRealDuneFolder(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "dune-awakening-selfhost-docker")
	for _, p := range []string{"runtime", "runtime/scripts"} {
		os.MkdirAll(filepath.Join(dir, p), 0o755)
	}
	os.WriteFile(filepath.Join(dir, "install.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644)
	// a fake dune CLI so the script's stop call is harmless
	os.WriteFile(filepath.Join(dir, "runtime/scripts/dune"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	sibling := filepath.Join(home, "other")
	os.MkdirAll(sibling, 0o755)

	out, code := run(t, home, uninstallAllScript(model.Default()))
	if code != 0 {
		t.Fatalf("expected success, got %d: %s", code, out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("the Dune Docker folder should be gone")
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatal("a neighbouring folder was deleted")
	}
}

func TestHyperVScriptPinsCommit(t *testing.T) {
	s := hypervScript("install")
	if !strings.Contains(s, HyperVFixCommit) || strings.Contains(s, "refs/heads/main") {
		t.Fatal("the fix must be fetched from the pinned commit, not a moving branch")
	}
	if len(HyperVFixCommit) != 40 {
		t.Fatalf("commit pin looks wrong: %q", HyperVFixCommit)
	}
}
