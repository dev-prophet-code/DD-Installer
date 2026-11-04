package install

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
)

// fakeRunner scripts responses by substring of the script text.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []string
	respond func(script string, n int) (string, int)
	count   map[string]int
}

func (f *fakeRunner) Run(_ context.Context, script string, out io.Writer) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, script)
	if f.count == nil {
		f.count = map[string]int{}
	}
	key := ""
	for _, k := range []string{"bootstrap.sh", "./install.sh", "api.py\" \"$T/params.json\" setup", "params.json\" start", "dune ready", "params.json\" layout", "90-dd-installer", "usermod"} {
		if strings.Contains(script, k) {
			key = k
			break
		}
	}
	f.count[key]++
	o, c := f.respond(key, f.count[key])
	io.WriteString(out, o)
	return c, nil
}
func (f *fakeRunner) Describe() string { return "fake" }
func (f *fakeRunner) Close() error     { return nil }

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

func TestHappyPath(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(string, int) (string, int) { return "ok\n", 0 }}
	if err := Run(context.Background(), baseOpts(r, &evs)); err != nil {
		t.Fatal(err)
	}
	last := evs[len(evs)-1]
	if last.Type != "done" {
		t.Fatalf("last event %v", last)
	}
	all := strings.Join(r.calls, "\n")
	for _, want := range []string{"DUNE_BOOTSTRAP_SKIP_INSTALL=1", "ADMIN_BIND_PORT=8088 ./install.sh", `"Alpha"`, `"instances":3`, `DUNE_PG_JIT`} {
		if !strings.Contains(all, want) {
			t.Errorf("scripts missing %q", want)
		}
	}
}

func TestAssistantFixesMissingToolsAndRetries(t *testing.T) {
	var evs []Event
	r := &fakeRunner{respond: func(k string, n int) (string, int) {
		if k == "bootstrap.sh" && n == 1 {
			return "Installation could not continue: tar is missing.\n", 1
		}
		return "ok\n", 0
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
		if k == "./install.sh" && n == 1 {
			return "Port 8088 is already in use. Choose another port.\n", 1
		}
		return "ok\n", 0
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
		if k == "./install.sh" && n == 1 {
			return "permission denied while trying to connect to /var/run/docker.sock\n", 1
		}
		return "ok\n", 0
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
		if k == "./install.sh" {
			return "Use this password to sign in:\n  s3cretGenPass\nfailed token=TOPSECRETTOKEN123\nPort 8088 is already in use.\n", 1
		}
		return "ok\n", 0
	}}
	err := Run(context.Background(), baseOpts(r, &evs))
	if err == nil {
		t.Fatal("persistent failure must end the run")
	}
	for _, e := range evs {
		if strings.Contains(e.Msg, "s3cretGenPass") || strings.Contains(e.Msg, "TOPSECRETTOKEN123") {
			t.Fatalf("secret leaked in event: %q", e.Msg)
		}
	}
	ports := 0
	for _, c := range r.calls {
		if strings.Contains(c, "./install.sh") {
			ports++
		}
	}
	if ports > 2 { // original + exactly one retry; the same rule is never re-applied
		t.Fatalf("fix repeated: %d install attempts", ports)
	}
}

func TestScriptsQuoteSafely(t *testing.T) {
	cfg := model.Default()
	cfg.InstallDir = "~/my dune"
	if !strings.Contains(bootstrapScript(cfg, false), `DIR="$HOME/my dune"`) {
		t.Fatal("dir expr")
	}
	s := tuningScript(cfg, map[string]string{"GOOD": "a b", "bad key": "x", "EVIL": "$(id)"})
	if strings.Contains(s, "$(id)") || strings.Contains(s, "bad key") {
		t.Fatal("unsafe env entry written")
	}
}
