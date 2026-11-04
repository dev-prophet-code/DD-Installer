package ui

import (
	"context"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

type fakePF struct {
	ls   platform.LocalSupport
	id   string
	prov int
}

func (f *fakePF) ID() string                          { return f.id }
func (f *fakePF) DisplayName() string                 { return "Fake" }
func (f *fakePF) LocalSupport() platform.LocalSupport { return f.ls }
func (f *fakePF) LocalRunner(model.Config) (transport.Runner, error) {
	return transport.NewLocal(), nil
}
func (f *fakePF) ProvisionLocal(context.Context, func(string)) (bool, error) {
	f.prov++
	return false, nil
}
func (f *fakePF) DefaultSSHKeys() []string { return nil }

func newTest(t *testing.T, pf *fakePF) *wizard {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	_, z := newWizard(a, app.New(pf))
	return z
}

func TestWizardWalkthrough(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})

	// Page 0: SSH validation blocks Next until filled.
	z.onNext()
	if z.cur != 0 || z.errLabel.Text == "" {
		t.Fatalf("empty SSH form must block: cur=%d err=%q", z.cur, z.errLabel.Text)
	}
	z.host.SetText("srv.example.com")
	z.user.SetText("dune")
	z.key.SetText("/tmp/key")
	z.onNext()
	if z.cur != 1 {
		t.Fatalf("expected page 1, err=%q", z.errLabel.Text)
	}

	// Page 1: token required.
	z.onNext()
	if z.cur != 1 || !strings.Contains(z.errLabel.Text, "token") {
		t.Fatalf("token must be required: %q", z.errLabel.Text)
	}
	z.token.SetText("TOKEN-123456")
	z.title.SetText("Arrakis")
	z.onNext()
	if z.cur != 2 {
		t.Fatalf("expected page 2: %q", z.errLabel.Text)
	}

	// Page 2: layout -> request reflects names, modes, DD with third role.
	z.addSietch("Atreides", "PVE")
	z.rows[1].mode.SetSelected("PVP")
	z.dd.SetSelected("3")
	z.dd3.SetSelected("PVP")
	req := z.request()
	if len(req.Config.Sietches) != 2 || req.Config.Sietches[1].Mode != model.PvP || req.Config.Sietches[1].Name != "Atreides" {
		t.Fatalf("sietches: %+v", req.Config.Sietches)
	}
	if req.Config.DeepDesert.Instances != 3 || req.Config.DeepDesert.ThirdRole != model.PvP {
		t.Fatalf("deep desert: %+v", req.Config.DeepDesert)
	}
	if !strings.Contains(z.ramLabel.Text, "GB") {
		t.Fatal("ram estimate missing")
	}
	z.rows[1].name.SetText("bad;name")
	z.onNext()
	if z.cur != 2 {
		t.Fatal("invalid sietch name must block")
	}
	z.rows[1].name.SetText("Atreides")
	z.onNext()
	if z.cur != 3 {
		t.Fatalf("expected page 3: %q", z.errLabel.Text)
	}

	// Page 3: Next blocked until a successful scan.
	z.onNext()
	if z.cur != 3 {
		t.Fatal("scan must be required")
	}
	z.scanned = true
	z.onNext()
	if z.cur != 4 {
		t.Fatal("expected install page")
	}

	// Page 4: the install button needs the disclaimer.
	if !z.install.Disabled() {
		t.Fatal("install must be disabled without accepting the disclaimer")
	}
	z.accept.SetChecked(true)
	if z.install.Disabled() {
		t.Fatal("install must be enabled after accepting")
	}
	z.accept.SetChecked(false)
	if err := z.c.Start(z.request()); err == nil {
		t.Fatal("controller must refuse without disclaimer")
	}
	if !strings.Contains(z.summary.Text, "Atreides [PVP]") || strings.Contains(z.summary.Text, "TOKEN") {
		t.Fatalf("summary: %q", z.summary.Text)
	}
}

func TestEventsRender(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	z.onEvent(install.Event{Type: "step", Step: "bootstrap", Status: "running", Msg: "Download"})
	z.onEvent(install.Event{Type: "step", Step: "bootstrap", Status: "ok", Msg: "Download"})
	z.onEvent(install.Event{Type: "log", Msg: "hello"})
	d := assistant.Diagnosis{RuleID: "x", Title: "Port busy", Explain: "e", Risk: assistant.RiskSafe,
		Actions: []assistant.Action{{Description: "use next port"}}}
	z.onEvent(install.Event{Type: "diagnosis", Diagnosis: &d})
	if len(z.progress.Objects) != 1 || len(z.assist.Objects) != 1 || !strings.Contains(z.logEntry.Text, "hello") {
		t.Fatal("events not rendered")
	}
	for i := 0; i < 1000; i++ {
		z.appendLog("line")
	}
	if len(z.logLines) > 600 {
		t.Fatal("log must be capped")
	}
	z.onEvent(install.Event{Type: "error", Msg: "boom"})
	if z.errLabel.Text != "boom" {
		t.Fatal("error not shown")
	}
}

func TestProvisionOfferedWhenWSLMissing(t *testing.T) {
	pf := &fakePF{id: "windows", ls: platform.LocalSupport{Reason: "No Ubuntu WSL2 distribution found.", CanProvision: true, ProvisionHint: "install it"}}
	z := newTest(t, pf)
	if !z.provisionBtn.Visible() {
		t.Fatal("WSL install offer must be visible when WSL/Ubuntu is missing")
	}
	// Choosing local without WSL must not let the wizard continue.
	z.target.SetSelected(optLocal)
	z.onNext()
	if z.cur != 0 || !strings.Contains(z.errLabel.Text, "not possible") {
		t.Fatalf("local without WSL must block: %q", z.errLabel.Text)
	}
	// Once WSL exists the offer disappears and local works.
	pf.ls = platform.LocalSupport{Supported: true, Distros: []string{"Ubuntu"}}
	z.refreshLocal()
	if z.provisionBtn.Visible() {
		t.Fatal("offer must disappear once Ubuntu is installed")
	}
	z.onNext()
	if z.cur != 1 {
		t.Fatalf("local install should proceed: %q", z.errLabel.Text)
	}
}

func TestMacOSHasNoLocalOption(t *testing.T) {
	z := newTest(t, &fakePF{id: "macos", ls: platform.LocalSupport{Reason: "x86-64 Linux needed"}})
	if z.provisionBtn.Visible() {
		t.Fatal("no provisioning on macOS")
	}
	z.target.SetSelected(optLocal)
	z.onNext()
	if z.cur != 0 {
		t.Fatal("local must be blocked on macOS")
	}
}
