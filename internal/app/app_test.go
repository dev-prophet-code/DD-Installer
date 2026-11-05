package app

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// fakeRunner is a target that reports a healthy 64 GB Ubuntu host and says
// "done / ok" to everything else.
type fakeRunner struct {
	mu       sync.Mutex
	scripts  []string
	exit     func(script string) int
	extraOut func(script string) string
	closed   bool
}

const probeOut = "os=Linux\narch=x86_64\nuid=1000\nuser=dune\ncpu_threads=16\navx=1\navx2=1\nmem_total_kb=67108864\ndisk_free_kb=500000000\nsystemd=1\nhas_curl=1\nhas_tar=1\ndocker=1\ncompose=1\n"

func (f *fakeRunner) Run(_ context.Context, script string, out io.Writer) (int, error) {
	f.mu.Lock()
	f.scripts = append(f.scripts, script)
	f.mu.Unlock()
	if strings.Contains(script, "DUNE_PROBE_DIR") {
		io.WriteString(out, probeOut)
		return 0, nil
	}
	if f.extraOut != nil {
		io.WriteString(out, f.extraOut(script))
	}
	if f.exit != nil {
		return f.exit(script), nil
	}
	return 0, nil
}
func (f *fakeRunner) Describe() string { return "fake-target" }
func (f *fakeRunner) Close() error     { f.closed = true; return nil }

type fakePF struct {
	run   *fakeRunner
	ls    platform.LocalSupport
	slept int
	rel   int
}

func (p *fakePF) ID() string                                         { return "linux" }
func (p *fakePF) DisplayName() string                                { return "Fake" }
func (p *fakePF) LocalSupport() platform.LocalSupport                { return p.ls }
func (p *fakePF) LocalRunner(model.Config) (transport.Runner, error) { return p.run, nil }
func (p *fakePF) ProvisionLocal(context.Context, func(string)) (bool, error) {
	return true, errors.New("not here")
}
func (p *fakePF) PreventSleep() func()     { p.slept++; return func() { p.rel++ } }
func (p *fakePF) DefaultSSHKeys() []string { return nil }

func newCtl(t *testing.T) (*Controller, *fakePF) {
	t.Helper()
	pf := &fakePF{run: &fakeRunner{}, ls: platform.LocalSupport{Supported: true}}
	return New(pf), pf
}

func goodRequest() Request {
	cfg := model.Default()
	cfg.Sietches = []model.Sietch{{Name: "Alpha", Mode: model.PvE}}
	return Request{Config: cfg, FuncomToken: strings.Repeat("aB1x", 12), AcceptDisclaimer: true}
}

func waitFor(t *testing.T, c *Controller, typ string) []install.Event {
	t.Helper()
	var mu sync.Mutex
	var got []install.Event
	done := make(chan struct{})
	var once sync.Once
	stop := c.Subscribe(func(e install.Event) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
		if e.Type == typ {
			once.Do(func() { close(done) })
		}
	})
	defer stop()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %q event", typ)
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]install.Event(nil), got...)
}

func TestValidateAndCfgSecrets(t *testing.T) {
	c, _ := newCtl(t)
	r := goodRequest()
	if errs := c.Validate(r); len(errs) != 0 {
		t.Fatalf("good request invalid: %v", errs)
	}
	r.SSHPassword, r.SSHPassphrase, r.FuncomToken = "pw", "pp", "  tok  \n"
	cfg := r.Cfg()
	if cfg.SSH.Password != "pw" || cfg.SSH.Passphrase != "pp" || cfg.FuncomToken != "tok" {
		t.Fatalf("secrets not applied/trimmed: %+v", cfg.SSH)
	}
}

func TestScanLocalAndFailures(t *testing.T) {
	c, pf := newCtl(t)
	res := c.Scan(context.Background(), goodRequest())
	if res.Err != "" || res.Target != "fake-target" || !res.Report.OK || res.Host.MemTotalKB == 0 {
		t.Fatalf("scan: %+v", res)
	}
	if res.Host.Raw != nil {
		t.Fatal("raw probe output must not be exposed")
	}

	pf.ls = platform.LocalSupport{Reason: "no WSL"}
	res = c.Scan(context.Background(), goodRequest())
	if res.Err != "no WSL" {
		t.Fatalf("unsupported local must explain itself: %+v", res)
	}

	r := goodRequest()
	r.Config.Target = model.TargetSSH // incomplete SSH settings
	if res := c.Scan(context.Background(), r); !strings.HasPrefix(res.Err, "ssh:") {
		t.Fatalf("incomplete ssh config must be reported first: %+v", res)
	}
}

func TestSanitizeStripsSecrets(t *testing.T) {
	cfg := model.Default()
	cfg.SSH.Password, cfg.FuncomToken = "hunter2hunter2", "TOKEN-123456"
	got := Sanitize("failed: hunter2hunter2 and TOKEN-123456 and abc", cfg)
	if strings.Contains(got, "hunter2") || strings.Contains(got, "TOKEN-123456") || !strings.Contains(got, "abc") {
		t.Fatalf("sanitize: %q", got)
	}
}

func TestStartRefusals(t *testing.T) {
	c, _ := newCtl(t)
	r := goodRequest()
	r.AcceptDisclaimer = false
	if err := c.Start(r); err == nil || !strings.Contains(err.Error(), "disclaimer") {
		t.Fatalf("must require the disclaimer: %v", err)
	}
	r = goodRequest()
	r.FuncomToken = ""
	if err := c.Start(r); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("must require the token: %v", err)
	}
	r = goodRequest()
	r.Config.WebPort = 0
	if err := c.Start(r); err == nil {
		t.Fatal("invalid config must be refused")
	}
}

func TestFullRunResumesEverythingAndKeepsComputerAwake(t *testing.T) {
	c, pf := newCtl(t)
	if err := c.Start(goodRequest()); err != nil {
		t.Fatal(err)
	}
	evs := waitFor(t, c, "done")
	if evs[len(evs)-1].Port == 0 {
		t.Fatal("the done event must carry the console port")
	}
	deadline := time.Now().Add(3 * time.Second)
	for c.Running() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c.Running() {
		t.Fatal("controller still running after done")
	}
	if pf.slept != 1 || pf.rel != 1 {
		t.Fatalf("sleep inhibition must be taken and released once: %d/%d", pf.slept, pf.rel)
	}
	if !pf.run.closed {
		t.Fatal("the target connection must be closed at the end")
	}
	// a second run while idle works (state is reset)
	if err := c.Start(goodRequest()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	waitFor(t, c, "done")
}

func TestStartRefusesSecondRunWhileRunning(t *testing.T) {
	c, pf := newCtl(t)
	block := make(chan struct{})
	pf.run.exit = func(script string) int {
		if strings.Contains(script, "install.sh") && strings.Contains(script, "api/health") {
			return 1 // not done, forces the real step
		}
		<-block
		return 0
	}
	if err := c.Start(goodRequest()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := c.Start(goodRequest()); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second start must be refused: %v", err)
	}
	c.Cancel()
	close(block)
}

func TestBlockingProblemStopsBeforeInstalling(t *testing.T) {
	c, pf := newCtl(t)
	pf.run = &fakeRunner{extraOut: nil}
	// override the probe: no AVX2
	pf.run.exit = func(string) int { return 0 }
	bad := &noAVXRunner{fakeRunner: pf.run}
	pf.ls = platform.LocalSupport{Supported: true}
	c.pf = &runnerPF{fakePF: pf, r: bad}
	if err := c.Start(goodRequest()); err != nil {
		t.Fatal(err)
	}
	evs := waitFor(t, c, "error")
	found := false
	for _, e := range evs {
		if e.Type == "error" && strings.Contains(e.Msg, "AVX") {
			found = true
		}
		if e.Type == "step" {
			t.Fatal("no step may run on a host that fails the hard requirements")
		}
	}
	if !found {
		t.Fatal("the AVX2 problem must be reported")
	}
}

func TestConfirmUnknownAndCancel(t *testing.T) {
	c, _ := newCtl(t)
	c.Confirm("nope", true) // must not panic or block
	c.Cancel()              // nothing running
}

func TestSubscribeReplaysInOrderAndStops(t *testing.T) {
	c, _ := newCtl(t)
	for i := 0; i < 5; i++ {
		c.emit(install.Event{Type: "log", Msg: string(rune('a' + i))})
	}
	var mu sync.Mutex
	var got []string
	stop := c.Subscribe(func(e install.Event) { mu.Lock(); got = append(got, e.Msg); mu.Unlock() })
	time.Sleep(100 * time.Millisecond)
	c.emit(install.Event{Type: "log", Msg: "f"})
	time.Sleep(100 * time.Millisecond)
	stop()
	c.emit(install.Event{Type: "log", Msg: "after-stop"})
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(got, "") != "abcdef" {
		t.Fatalf("history must be replayed once and in order, got %q", strings.Join(got, ""))
	}
}

func TestRunAction(t *testing.T) {
	c, pf := newCtl(t)
	var lines []string
	log := func(l string) { lines = append(lines, l) }
	pf.run.extraOut = func(string) string { return "hello from the server\nDD_PORT 1\n" }

	if _, err := c.RunAction(context.Background(), goodRequest(), "nope", "", log); err == nil {
		t.Fatal("unknown action must fail")
	}
	if _, err := c.RunAction(context.Background(), goodRequest(), "uninstall-all", "delete all", log); err == nil || !strings.Contains(err.Error(), "DELETE ALL") {
		t.Fatalf("destructive actions need the exact phrase: %v", err)
	}
	if len(pf.run.scripts) != 0 {
		t.Fatal("nothing may run before the confirmation")
	}
	code, err := c.RunAction(context.Background(), goodRequest(), "status", "", log)
	if err != nil || code != 0 {
		t.Fatalf("status: %d %v", code, err)
	}
	if strings.Join(lines, "|") != "hello from the server" {
		t.Fatalf("output must stream without marker lines: %v", lines)
	}
	if pf.slept != 1 || pf.rel != 1 {
		t.Fatal("long actions keep the computer awake too")
	}
	if _, err := c.RunAction(context.Background(), goodRequest(), "uninstall-all", "DELETE ALL", log); err != nil {
		t.Fatalf("the exact phrase must be accepted: %v", err)
	}
}

func TestRunActionRefusedWhileRunning(t *testing.T) {
	c, _ := newCtl(t)
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
	if _, err := c.RunAction(context.Background(), goodRequest(), "status", "", func(string) {}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("must not overlap with another operation: %v", err)
	}
}

func TestFinishInfo(t *testing.T) {
	c, pf := newCtl(t)
	pf.run.extraOut = func(string) string { return "  s3cr3t-admin-pw\n" }
	info, err := c.FinishInfo(context.Background(), goodRequest(), 8089)
	if err != nil || info.Password != "s3cr3t-admin-pw" || info.URL != "http://localhost:8089" || info.Port != 8089 {
		t.Fatalf("local: %+v %v", info, err)
	}
	// an unreadable password file still yields the address, plus a helpful error
	pf.run.exit = func(string) int { return 1 }
	info, err = c.FinishInfo(context.Background(), goodRequest(), 0)
	if err == nil || info.URL == "" || info.Password != "" || info.Port != 8088 {
		t.Fatalf("failure path: %+v %v", info, err)
	}
}

func TestConsoleURL(t *testing.T) {
	cfg := model.Default()
	cfg.Target = model.TargetSSH
	cfg.SSH.Host = "2001:db8::1"
	if got := consoleURL(cfg, 8088); got != "http://[2001:db8::1]:8088" {
		t.Fatalf("ipv6: %s", got)
	}
	cfg.SSH.Host = "srv.example.com"
	if got := consoleURL(cfg, 9000); got != "http://srv.example.com:9000" {
		t.Fatalf("host: %s", got)
	}
}

func TestProvisionAndPlatformPassThrough(t *testing.T) {
	c, _ := newCtl(t)
	if reboot, err := c.ProvisionLocal(context.Background(), func(string) {}); !reboot || err == nil {
		t.Fatal("ProvisionLocal must pass the platform result through")
	}
	if c.Platform() == nil {
		t.Fatal("platform")
	}
	if _, err := New(nil).ProvisionLocal(context.Background(), nil); err == nil {
		t.Fatal("no platform must be an error")
	}
	if fp, err := c.HostKey(context.Background(), model.SSHConfig{}); err == nil || fp != "" {
		t.Fatal("host key lookup needs host and port")
	}
}

func TestDisclaimerTextsMentionTheEssentials(t *testing.T) {
	for _, want := range []string{"RedBlink", "Prophet", "credentials"} {
		if !strings.Contains(DisclaimerShort, want) || !strings.Contains(DisclaimerFull, want) {
			t.Errorf("disclaimer must mention %q", want)
		}
	}
}

// --- helpers for the blocking-problem test

type noAVXRunner struct{ *fakeRunner }

func (n *noAVXRunner) Run(ctx context.Context, script string, out io.Writer) (int, error) {
	if strings.Contains(script, "DUNE_PROBE_DIR") {
		io.WriteString(out, strings.Replace(strings.Replace(probeOut, "avx2=1", "avx2=0", 1), "avx=1", "avx=0", 1))
		return 0, nil
	}
	return n.fakeRunner.Run(ctx, script, out)
}

type runnerPF struct {
	*fakePF
	r transport.Runner
}

func (p *runnerPF) LocalRunner(model.Config) (transport.Runner, error) { return p.r, nil }

// The disclaimer has one source; the docs copy must not drift.
func TestDisclaimerDocIsInSync(t *testing.T) {
	doc, err := os.ReadFile("../../docs/DISCLAIMER.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(doc) != DisclaimerMarkdown() {
		t.Fatal("docs/DISCLAIMER.md differs from internal/app/disclaimer.md - run scripts/sync-disclaimer.sh")
	}
}

func TestDisclaimerPlainTextHasNoMarkup(t *testing.T) {
	for _, bad := range []string{"**", "`", "## ", "\n* "} {
		if strings.Contains(DisclaimerFull, bad) {
			t.Errorf("plain text still contains markup %q", bad)
		}
	}
	for _, want := range []string{"CREDENTIALS", "NO WARRANTY, NO LIABILITY", "Remove everything", "RedBlink", "Prophet"} {
		if !strings.Contains(DisclaimerFull, want) {
			t.Errorf("disclaimer must contain %q", want)
		}
	}
}
