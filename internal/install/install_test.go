package install

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

type transportRunner = transport.Runner

// fakeRunner plays the role of the target server. It classifies each script
// into a key and answers from respond; detached jobs are simulated.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []string
	respond func(key string, n int) (string, int)
	// fail returns a transport error for (key, nth call of that key), or nil
	fail  func(key string, n int) error
	count map[string]int
	jobs  map[string]*fakeJob
}

type fakeJob struct {
	out  string
	code int
}

var offsetRe = regexp.MustCompile(`tail -c \+(\d+)`)
var detachIDRe = regexp.MustCompile(`\.dd-installer/([a-z-]+)`)

func classify(script string) string {
	switch {
	case strings.Contains(script, "DD_STATE"):
		return "poll"
	case strings.Contains(script, "DD_DETACH started"):
		return "detach-start"
	case strings.Contains(script, "kill -TERM"):
		return "detach-kill"
	case strings.HasPrefix(script, "[ -f \"") && strings.Contains(script, "/install.sh\" ]"):
		return "check:bootstrap"
	case strings.Contains(script, "api/health") && strings.Contains(script, "DD_PORT"):
		return "check:install"
	case strings.Contains(script, "params.json\" check-setup"):
		return "check:setup"
	case strings.Contains(script, "Battlegroup is ready."):
		return "ready"
	case strings.Contains(script, "dune ready >/dev/null 2>&1"):
		return "check:ready"
	case strings.Contains(script, "Downloading Dune Docker") || strings.Contains(script, "bootstrap.sh"):
		return "bootstrap"
	case strings.Contains(script, "./install.sh"):
		return "install"
	case strings.Contains(script, "params.json\" setup"):
		return "console-setup"
	case strings.Contains(script, "params.json\" start"):
		return "start"
	case strings.Contains(script, "params.json\" layout"):
		return "layout"
	case strings.Contains(script, "90-dd-installer"):
		return "sudo"
	case strings.Contains(script, "usermod"):
		return "usermod"
	}
	return "other"
}

func (f *fakeRunner) Run(_ context.Context, script string, out io.Writer) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, script)
	if f.count == nil {
		f.count = map[string]int{}
		f.jobs = map[string]*fakeJob{}
	}
	key := classify(script)
	f.count[key]++
	if f.fail != nil {
		if err := f.fail(key, f.count[key]); err != nil {
			return -1, err
		}
	}
	switch key {
	case "detach-start":
		id := detachIDRe.FindStringSubmatch(script)[1]
		inner := classify(strings.Replace(script, "DD_DETACH started", "", 1))
		f.count[inner]++
		o, c := f.answer(inner)
		f.jobs[id] = &fakeJob{out: o, code: c}
		io.WriteString(out, "DD_DETACH started\n")
		return 0, nil
	case "poll":
		id := detachIDRe.FindStringSubmatch(script)[1]
		j := f.jobs[id]
		off := 0
		if m := offsetRe.FindStringSubmatch(script); m != nil {
			off, _ = strconv.Atoi(m[1])
			off--
		}
		if j == nil {
			io.WriteString(out, "DD_STATE gone\nDD_BODY\n")
			return 0, nil
		}
		body := ""
		if off < len(j.out) {
			body = j.out[off:]
		}
		fmt.Fprintf(out, "DD_STATE exit:%d\nDD_BODY\n%s", j.code, body)
		return 0, nil
	}
	o, c := f.answer(key)
	io.WriteString(out, o)
	return c, nil
}

func (f *fakeRunner) answer(key string) (string, int) {
	n := f.count[key]
	return f.respond(key, n)
}

func (f *fakeRunner) Describe() string { return "fake" }
func (f *fakeRunner) Close() error     { return nil }

func okAll(key string, _ int) (string, int) {
	if strings.HasPrefix(key, "check:") {
		return "", 1 // not done yet unless a test says so
	}
	return "ok\n", 0
}

func baseOpts(r *fakeRunner, evs *[]Event) Options {
	cfg := model.Default()
	cfg.FuncomToken = "TOPSECRETTOKEN123"
	cfg.Sietches = []model.Sietch{{Name: "Alpha", Mode: model.PvE}, {Name: "Beta", Mode: model.PvP}}
	cfg.DeepDesert = model.DeepDesert{Instances: 3, ThirdRole: model.PvP}
	return Options{
		Cfg: cfg, Runner: r, Host: probe.Info{User: "dune"},
		Report: sizing.Report{Env: map[string]string{"DUNE_PG_JIT": "off", "DUNE_RMQ_ERL_ARGS": "+S 2:2"}, PostCommands: []string{"runtime/scripts/dune memory-swap enable"}},
		Emit:   func(e Event) { *evs = append(*evs, e) },
		Sleep:  func(time.Duration) {},
	}
}

func stepStatus(evs []Event, id string) string {
	last := ""
	for _, e := range evs {
		if e.Type == "step" && e.Step == id {
			last = e.Status
		}
	}
	return last
}

func TestHappyPath(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) { return okAll(k, n) }}
	if err := Run(context.Background(), baseOpts(r, &evs)); err != nil {
		t.Fatal(err)
	}
	if evs[len(evs)-1].Type != "done" {
		t.Fatalf("last event %v", evs[len(evs)-1])
	}
	all := strings.Join(r.calls, "\n")
	for _, want := range []string{UpstreamTag, "ADMIN_BIND_PORT=8088 ./install.sh", `"Alpha"`, `"instances":3`, `DUNE_PG_JIT`, "setsid"} {
		if !strings.Contains(all, want) {
			t.Errorf("scripts missing %q", want)
		}
	}
	if strings.Contains(all, "DUNE_BOOTSTRAP_SKIP_INSTALL") {
		t.Error("the pinned release must be used by default, not upstream's 'latest' bootstrap")
	}
}

func TestLatestUpstreamUsesBootstrap(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) { return okAll(k, n) }}
	o := baseOpts(r, &evs)
	o.Cfg.UpstreamLatest = true
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r.calls, "\n"), "DUNE_BOOTSTRAP_SKIP_INSTALL=1") {
		t.Fatal("latest mode must use upstream bootstrap.sh")
	}
}

func TestResumeSkipsFinishedStepsAndLearnsPort(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		switch k {
		case "check:bootstrap", "check:setup", "check:ready":
			return "yes\n", 0
		case "check:install":
			return "DD_PORT 9001\nDD_UPSTREAM_VERSION " + UpstreamTag + "\n", 0
		}
		return okAll(k, n)
	}}
	if err := Run(context.Background(), baseOpts(r, &evs)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"bootstrap", "install", "console-setup", "start", "ready"} {
		if stepStatus(evs, id) != "skipped" {
			t.Errorf("step %s must be skipped when already done, got %q", id, stepStatus(evs, id))
		}
	}
	if stepStatus(evs, "layout") != "ok" {
		t.Fatal("layout must still run")
	}
	if r.count["install"] != 0 || r.count["detach-start"] != 0 {
		t.Fatal("install must not run again")
	}
	if !strings.Contains(strings.Join(r.calls, "\n"), `"port": 9001`) && !strings.Contains(strings.Join(r.calls, "\n"), `"port":9001`) {
		t.Fatal("later steps must use the port found on the server")
	}
}

func TestConnectionLossIsRetriedAfterReconnect(t *testing.T) {
	var evs []Event
	r := &fakeRunner{
		respond: func(k string, n int) (string, int) { return okAll(k, n) },
		fail: func(k string, n int) error {
			if k == "layout" && n == 1 {
				return io.EOF
			}
			return nil
		},
	}
	o := baseOpts(r, &evs)
	reconnects := 0
	o.Reconnect = func(context.Context) (transportRunner, error) { reconnects++; return r, nil }
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if reconnects != 1 || r.count["layout"] != 2 {
		t.Fatalf("expected one reconnect and a repeated layout step: reconnects=%d layout=%d", reconnects, r.count["layout"])
	}
}

func TestDetachedStepSurvivesDroppedPoll(t *testing.T) {
	var evs []Event
	polls := 0
	r := &fakeRunner{
		respond: func(k string, n int) (string, int) {
			if k == "install" {
				return "installing docker...\nDD_PORT 8088\nDD_UPSTREAM_VERSION " + UpstreamTag + "\n", 0
			}
			return okAll(k, n)
		},
		fail: func(k string, n int) error {
			if k == "poll" {
				polls++
				if polls == 1 {
					return fmt.Errorf("read tcp: connection reset by peer")
				}
			}
			return nil
		},
	}
	o := baseOpts(r, &evs)
	reconnects := 0
	o.Reconnect = func(context.Context) (transportRunner, error) { reconnects++; return r, nil }
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if reconnects != 1 || r.count["detach-start"] != 1 {
		t.Fatalf("the step must be started once and followed across the reconnect (reconnects=%d starts=%d)", reconnects, r.count["detach-start"])
	}
	seen := false
	for _, e := range evs {
		if e.Type == "log" && strings.Contains(e.Msg, "installing docker") {
			seen = true
		}
		if e.Type == "log" && strings.HasPrefix(e.Msg, "DD_") {
			t.Fatalf("marker line leaked to the log: %q", e.Msg)
		}
	}
	if !seen {
		t.Fatal("log of the detached step not shown")
	}
}

func TestVersionMismatchWarns(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		if k == "install" {
			return "DD_PORT 8088\nDD_UPSTREAM_VERSION v9.9.9\n", 0
		}
		return okAll(k, n)
	}}
	if err := Run(context.Background(), baseOpts(r, &evs)); err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.Type == "log" && strings.Contains(e.Msg, "WARNING") && strings.Contains(e.Msg, "v9.9.9") {
			return
		}
	}
	t.Fatal("expected a compatibility warning")
}

func TestAssistantFixesMissingToolsAndRetries(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		if k == "bootstrap" && n == 1 {
			return "Installation could not continue: tar is missing.\n", 1
		}
		return okAll(k, n)
	}}
	if err := Run(context.Background(), baseOpts(r, &evs)); err != nil {
		t.Fatal(err)
	}
	var sawDiag, sawFix bool
	for _, e := range evs {
		if e.Type == "diagnosis" && e.Diagnosis.RuleID == "missing-tools" {
			sawDiag = true
		}
		if e.Type == "fix" {
			sawFix = true
		}
	}
	if !sawDiag || !sawFix {
		t.Fatalf("assistant did not act: diag=%v fix=%v", sawDiag, sawFix)
	}
}

func TestPortConflictPicksNextPort(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		if k == "install" && n == 1 {
			return "Port 8088 is already in use. Choose another port.\n", 1
		}
		return okAll(k, n)
	}}
	if err := Run(context.Background(), baseOpts(r, &evs)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r.calls, "\n"), "ADMIN_BIND_PORT=8089 ./install.sh") {
		t.Fatal("port was not bumped")
	}
}

func TestModerateFixNeedsConfirmation(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		if k == "install" && n == 1 {
			return "permission denied while trying to connect to /var/run/docker.sock\n", 1
		}
		return okAll(k, n)
	}}
	o := baseOpts(r, &evs)
	asked := false
	o.Confirm = func(_ context.Context, _ string, d assistant.Diagnosis) bool { asked = true; return false }
	if err := Run(context.Background(), o); err == nil {
		t.Fatal("declined fix must fail the run")
	}
	if !asked || r.count["usermod"] != 0 {
		t.Fatal("moderate fix ran without consent")
	}
}

func TestSecretsRedactedAndFixOnce(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		if k == "install" {
			return "Use this password to sign in:\n  s3cretGenPass\nfailed token=TOPSECRETTOKEN123\nPort 8088 is already in use.\n", 1
		}
		return okAll(k, n)
	}}
	err := Run(context.Background(), baseOpts(r, &evs))
	if err == nil {
		t.Fatal("persistent failure must end the run")
	}
	if !strings.Contains(err.Error(), "Press Install again") && !strings.Contains(err.Error(), "press Install again") {
		t.Fatalf("the error must explain how to continue: %v", err)
	}
	for _, e := range evs {
		if strings.Contains(e.Msg, "s3cretGenPass") || strings.Contains(e.Msg, "TOPSECRETTOKEN123") {
			t.Fatalf("secret leaked in event: %q", e.Msg)
		}
	}
	if r.count["detach-start"] > 2 { // original + exactly one retry; the same rule is never re-applied
		t.Fatalf("fix repeated: %d install attempts", r.count["detach-start"])
	}
}

func TestScriptsQuoteSafely(t *testing.T) {
	cfg := model.Default()
	cfg.InstallDir = "~/my dune"
	if !strings.Contains(bootstrapScript(cfg, false), `DIR="$HOME/my dune"`) {
		t.Fatal("dir expr")
	}
	if !strings.Contains(pinnedBootstrapScript(cfg), `DIR="$HOME/my dune"`) {
		t.Fatal("pinned dir expr")
	}
	s := tuningScript(cfg, map[string]string{"GOOD": "a b", "bad key": "x", "EVIL": "$(id)"})
	if strings.Contains(s, "$(id)") || strings.Contains(s, "bad key") {
		t.Fatal("unsafe env entry written")
	}
}
