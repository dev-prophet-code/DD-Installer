package install

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

func runSh(t *testing.T, script string, env ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", "-c", `eval "$(cat)"`)
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// fakeBin creates stub programs (sudo, visudo, ...) that only record their use.
func fakeBin(t *testing.T, names ...string) (dir, log string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "bin")
	log = filepath.Join(t.TempDir(), "calls.log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		body := "#!/bin/sh\ncat >/dev/null 2>&1 </dev/null || true\necho \"" + n + " $*\" >> '" + log + "'\nexit 0\n"
		if n == "rm" { // keep the real temp file, so the test can read what would have gone into sudoers
			body = "#!/bin/sh\nexit 0\n"
		}
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir, log
}

// The SSH user ends up in /etc/sudoers.d. A hostile name must never run code
// or add a second sudoers line.
func TestSudoGrantRefusesHostileUsers(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "PWNED")
	for _, u := range []string{
		"x'; touch " + marker + "; echo '",
		"dune\nmallory",
		"dune ALL=(ALL) ALL",
		"$(touch " + marker + ")",
		"root", "", "a b",
	} {
		s := sudoGrantScript(u)
		if model.ValidUser(u) == nil && u != "root" {
			t.Fatalf("test input %q should be invalid", u)
		}
		if u == "root" { // ValidUser rejects root as well
			if model.ValidUser(u) == nil {
				t.Fatal("root must be rejected")
			}
		}
		out, err := runSh(t, "DD_SUDO_PW=pw\nexport DD_SUDO_PW\n"+s)
		if err == nil || !strings.Contains(out, "refuses") {
			t.Errorf("user %q: expected a refusal, got err=%v out=%q", u, err, out)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("user %q executed code", u)
		}
	}
}

func TestSudoGrantWritesExactlyOneLine(t *testing.T) {
	bin, log := fakeBin(t, "sudo", "visudo", "install", "systemd-run", "rm")
	out, err := runSh(t, "DD_SUDO_PW=pw\nexport DD_SUDO_PW\n"+sudoGrantScript("dune"), "PATH="+bin+":"+os.Getenv("PATH"))
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	rec, _ := os.ReadFile(log)
	m := regexp.MustCompile(`visudo -cf '([^']+)'`).FindStringSubmatch(string(rec))
	if m == nil {
		t.Fatalf("visudo check not run: %s", rec)
	}
	b, _ := os.ReadFile(m[1])
	if string(b) != "dune ALL=(ALL) NOPASSWD:ALL\n" {
		t.Fatalf("sudoers content = %q", b)
	}
	s := sudoGrantScript("dune")
	for _, want := range []string{"--on-active=3h", sudoBootUnit, "WantedBy=multi-user.target", "ExecStart=/bin/rm -f /etc/sudoers.d/90-dd-installer"} {
		if !strings.Contains(s, want) {
			t.Errorf("grant script lacks %q", want)
		}
	}
}

// The grant file is removed LAST, because every earlier cleanup command still
// needs the passwordless sudo.
func TestSudoRevokeRemovesTheGrantLast(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(sudoRevokeScript), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "rm -f /etc/sudoers.d/90-dd-installer") {
		t.Fatalf("last line is %q", last)
	}
	if !strings.Contains(sudoRevokeScript, sudoBootUnit) {
		t.Fatal("the boot-time safety net must be removed too")
	}
	if !strings.Contains(sudoRevokedCheckScript, "sudo -k") || !strings.Contains(sudoRevokedCheckScript, "sudo -n true") {
		t.Fatal("the revocation check must drop cached credentials and test sudo -n")
	}
}

func TestLockSudoVerifiesTheRevocation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		checkExit  int
		wantLocked bool
		wantLog    string
	}{{"revoked", 0, true, "removed and verified"}, {"still active", 1, false, "WARNING"}} {
		t.Run(tc.name, func(t *testing.T) {
			var evs []Event
			r := &fakeRunner{respond: func(k string, _ int) (string, int) {
				if k == "other" { // the "is sudo gone?" check
					return "", tc.checkExit
				}
				return "ok\n", 0
			}}
			o := baseOpts(r, &evs)
			st := &state{sudoGrant: true}
			o.lockSudo(st)
			if (!st.sudoGrant) != tc.wantLocked {
				t.Fatalf("sudoGrant=%v", st.sudoGrant)
			}
			found := false
			for _, e := range evs {
				if strings.Contains(e.Msg, tc.wantLog) {
					found = true
				}
			}
			if !found {
				t.Fatalf("no %q message in %v", tc.wantLog, evs)
			}
		})
	}
	// nothing granted -> nothing is sent to the server
	r := &fakeRunner{respond: okAll}
	var evs []Event
	o := baseOpts(r, &evs)
	o.lockSudo(&state{})
	if len(r.calls) != 0 {
		t.Fatal("lockSudo must be a no-op without a grant")
	}
}

// The installation is verified by the last step, and the sudo access is locked
// before "done" is reported.
func TestRunLocksSudoRightAfterVerification(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		switch k {
		case "install":
			if n == 1 {
				return "sudo: a password is required\n", 1
			}
		case "check:ready", "check:install", "check:setup", "check:bootstrap":
			if k == "check:ready" {
				return "", 0 // the verification step and the ready checks pass
			}
			return "", 1
		}
		return okAll(k, n)
	}}
	o := baseOpts(r, &evs)
	o.Cfg.Target = model.TargetSSH
	o.Cfg.SSH = model.SSHConfig{Host: "h", Port: 22, User: "dune", Auth: model.AuthPassword, Password: "pw12345"}
	o.Confirm = func(context.Context, string, assistant.Diagnosis) bool { return true }
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if stepStatus(evs, "finalize") != "ok" {
		t.Fatalf("finalize step: %q", stepStatus(evs, "finalize"))
	}
	lockAt, doneAt := -1, -1
	for i, e := range evs {
		if strings.Contains(e.Msg, "removed and verified") {
			lockAt = i
		}
		if e.Type == "done" {
			doneAt = i
		}
	}
	if lockAt < 0 || doneAt < 0 || lockAt > doneAt {
		t.Fatalf("sudo must be locked and verified before 'done' (lock=%d done=%d)", lockAt, doneAt)
	}
}

// The helper talks to the local console. An http_proxy from the environment
// must NOT receive the request (it would carry the admin password and token).
func TestConsoleHelperBypassesEnvironmentProxy(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		if py, err = exec.LookPath("python"); err != nil {
			t.Skip("python is not installed")
		}
	}
	var console, proxy atomic.Int32
	cs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		console.Add(1)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"files":{"complete":true}}`))
	}))
	defer cs.Close()
	ps := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { proxy.Add(1) }))
	defer ps.Close()

	dir := t.TempDir()
	port := cs.Listener.Addr().(interface{ String() string }).String()
	port = port[strings.LastIndex(port, ":")+1:]
	_ = os.WriteFile(filepath.Join(dir, "api.py"), []byte(consoleHelper), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "params.json"), []byte(`{"port":`+port+`,"passwordFile":"x"}`), 0o600)
	cmd := exec.Command(py, filepath.Join(dir, "api.py"), filepath.Join(dir, "params.json"), "check-setup")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "http_proxy=" + ps.URL, "HTTP_PROXY=" + ps.URL}
	out, _ := cmd.CombinedOutput()
	if console.Load() == 0 || proxy.Load() != 0 {
		t.Fatalf("console hits=%d, PROXY hits=%d\n%s", console.Load(), proxy.Load(), out)
	}
	if !strings.Contains(string(out), "Proxy settings found") {
		t.Errorf("the helper should say that it ignored the proxy: %s", out)
	}
}

// The job folder holds the full log, including the generated admin password.
func TestDetachedJobFolderIsDeletedWhenFinished(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	o := &Options{Runner: &transport.Local{Cmd: transport.ShellArgs}, Emit: func(Event) {},
		Sleep: func(time.Duration) { time.Sleep(100 * time.Millisecond) }}
	_, _, err := runDetached(context.Background(), o, newRedactor(), "install", "echo 'Use this password to sign in'\necho SECRETPASSWORD\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".dd-installer", "install")); err == nil {
		t.Fatal("the job folder with the log must be removed after the step")
	}
	// while it runs, the folder is private
	s := detachStartScript("install", "echo x\n")
	if !strings.Contains(s, "umask 077") || !strings.Contains(s, `chmod 700 "$HOME/.dd-installer"`) {
		t.Fatal("the job folder must be created private")
	}
}
