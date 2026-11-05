package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/sshkey"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

type fakePF struct {
	ls   platform.LocalSupport
	id   string
	name string
	prov int
}

func (f *fakePF) ID() string { return f.id }
func (f *fakePF) DisplayName() string {
	if f.name != "" {
		return f.name
	}
	return "Fake"
}
func (f *fakePF) LocalSupport() platform.LocalSupport { return f.ls }
func (f *fakePF) LocalRunner(model.Config) (transport.Runner, error) {
	return transport.NewLocal(), nil
}
func (f *fakePF) ProvisionLocal(context.Context, func(string)) (bool, error) {
	f.prov++
	return false, nil
}
func (f *fakePF) PreventSleep() func()     { return func() {} }
func (f *fakePF) DefaultSSHKeys() []string { return nil }

func newTest(t *testing.T, pf *fakePF) *wizard {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // no real ~/.ssh/config
	t.Setenv("USERPROFILE", t.TempDir())
	a := test.NewApp()
	t.Cleanup(a.Quit)
	_, z := newWizard(a, app.New(pf))
	return z
}

func goodToken() string { return strings.Repeat("a1B2", 12) }

func TestWizardWalkthrough(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	if z.cur != pgWelcome {
		t.Fatal("the wizard starts on the welcome page")
	}
	z.onNext()
	if z.cur != pgTarget {
		t.Fatal("welcome -> target")
	}

	// Target: SSH validation blocks Next until filled, with field-level messages.
	z.onNext()
	if z.cur != pgTarget || !strings.Contains(z.errLabel.Text, "Host") {
		t.Fatalf("empty SSH form must block: cur=%d err=%q", z.cur, z.errLabel.Text)
	}
	z.tgt.host.SetText("srv.example.com")
	z.tgt.user.SetText("root")
	z.tgt.key.SetText("/tmp/key")
	z.onNext()
	if z.cur != pgTarget || !strings.Contains(z.errLabel.Text, "root") {
		t.Fatalf("root must be refused inline: %q", z.errLabel.Text)
	}
	z.tgt.user.SetText("dune")
	z.onNext()
	if z.cur != pgServer {
		t.Fatalf("expected server page, err=%q", z.errLabel.Text)
	}

	// Server: token required and checked.
	z.onNext()
	if z.cur != pgServer || !strings.Contains(z.errLabel.Text, "token") {
		t.Fatalf("token must be required: %q", z.errLabel.Text)
	}
	z.srv.token.SetText("has space in it")
	z.onNext()
	if z.cur != pgServer {
		t.Fatal("a token with spaces must be refused")
	}
	z.srv.token.SetText(goodToken())
	z.srv.title.SetText("Arrakis")
	z.srv.latest.SetChecked(true)
	if !z.request().Config.UpstreamLatest {
		t.Fatal("newest-upstream choice must reach the config")
	}
	z.srv.latest.SetChecked(false)
	z.onNext()
	if z.cur != pgLayout {
		t.Fatalf("expected layout page: %q", z.errLabel.Text)
	}

	// Layout.
	z.addSietch("Atreides", "PVE")
	z.lay.rows[1].mode.SetSelected("PVP")
	z.lay.dd.SetSelected("3")
	z.lay.dd3.SetSelected("PVP")
	req := z.request()
	if len(req.Config.Sietches) != 2 || req.Config.Sietches[1].Mode != model.PvP || req.Config.Sietches[1].Name != "Atreides" {
		t.Fatalf("sietches: %+v", req.Config.Sietches)
	}
	if req.Config.DeepDesert.Instances != 3 || req.Config.DeepDesert.ThirdRole != model.PvP {
		t.Fatalf("deep desert: %+v", req.Config.DeepDesert)
	}
	z.lay.rows[1].name.SetText("bad;name")
	z.onNext()
	if z.cur != pgLayout || !strings.Contains(z.errLabel.Text, "Sietch 2") {
		t.Fatalf("invalid sietch name must block and name the row: %q", z.errLabel.Text)
	}
	z.lay.rows[1].name.SetText("Atreides")
	z.onNext()
	if z.cur != pgScan {
		t.Fatalf("expected scan page: %q", z.errLabel.Text)
	}

	// Scan: firewall options exist for SSH targets.
	if !z.scn.firewall.Visible() {
		t.Fatal("firewall option belongs to remote servers")
	}
	z.scn.firewall.SetChecked(true)
	z.scn.allowFrom.SetText("nonsense")
	z.scn.scanned = true
	z.onNext()
	if z.cur != pgScan {
		t.Fatal("an invalid allow-from value must block")
	}
	z.scn.allowFrom.SetText("203.0.113.0/24")
	got := z.request().Config
	if !got.ConfigureFirewall || got.ConsoleAllowFrom != "203.0.113.0/24" {
		t.Fatalf("firewall settings lost: %+v", got)
	}
	z.onNext()
	if z.cur != pgInstall {
		t.Fatalf("expected install page: %q", z.errLabel.Text)
	}

	// Install: needs the disclaimer.
	if !z.ins.install.Disabled() {
		t.Fatal("install must be disabled without accepting the disclaimer")
	}
	z.ins.accept.SetChecked(true)
	if z.ins.install.Disabled() {
		t.Fatal("install must be enabled after accepting")
	}
	z.ins.accept.SetChecked(false)
	if err := z.c.Start(z.request()); err == nil {
		t.Fatal("controller must refuse without disclaimer")
	}
	s := z.ins.summary.Text
	if !strings.Contains(s, "Atreides [PVP]") || strings.Contains(s, "TOKEN") || strings.Contains(s, goodToken()) || !strings.Contains(s, install.UpstreamTag) {
		t.Fatalf("summary: %q", s)
	}
}

func TestFirewallOptionsHiddenForLocal(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	z.tgt.target.SetSelected(optLocal)
	if z.scn.firewall.Visible() {
		t.Fatal("a local install has no server firewall to configure")
	}
}

func TestEventsRender(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	z.onEvent(install.Event{Type: "step", Step: "bootstrap", Status: "running", Msg: "Download"})
	z.onEvent(install.Event{Type: "step", Step: "bootstrap", Status: "ok", Msg: "Download"})
	z.onEvent(install.Event{Type: "log", Msg: "hello"})
	z.renderLog() // log drawing is coalesced; draw now
	d := assistant.Diagnosis{RuleID: "x", Title: "Port busy", Explain: "e", Risk: assistant.RiskSafe,
		Actions: []assistant.Action{{Description: "use next port"}}, DocURL: "https://docs.dunedocker.app/"}
	z.onEvent(install.Event{Type: "diagnosis", Diagnosis: &d})
	if len(z.ins.progress.Objects) != 1 || len(z.ins.assist.Objects) != 1 || !strings.Contains(z.ins.logEntry.Text, "hello") {
		t.Fatal("events not rendered")
	}
	if z.ins.overall.Value <= 0 {
		t.Fatal("the overall progress bar must advance when a step finishes")
	}
	z.onEvent(install.Event{Type: "error", Msg: "boom"})
	if z.errLabel.Text != "boom" {
		t.Fatal("error not shown")
	}
}

func TestLogFilterFollowAndReadOnly(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	for i := 0; i < 7000; i++ {
		z.appendLog("line")
	}
	z.appendLog("docker pull ok")
	z.renderLog()
	if len(z.ins.logAll) > maxLogLines {
		t.Fatalf("log memory must be capped, got %d", len(z.ins.logAll))
	}
	if n := strings.Count(z.ins.logEntry.Text, "\n") + 1; n > shownLogRows {
		t.Fatalf("rendered rows must be capped, got %d", n)
	}
	z.ins.logFind.SetText("DOCKER")
	if z.ins.logEntry.Text != "docker pull ok" {
		t.Fatalf("filter (case-insensitive) failed: %q", z.ins.logEntry.Text)
	}
	z.ins.logFind.SetText("")
	z.ins.logEntry.SetText("user typed something") // the log is read-only
	if z.ins.logEntry.Text == "user typed something" {
		t.Fatal("typing into the log must be undone")
	}
}

func TestProgressBarFromLog(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	z.onEvent(install.Event{Type: "log", Msg: "Update state (0x61) downloading, progress: 42.50 (1 / 2)"})
	if !z.ins.detail.Visible() || z.ins.detail.Value < 0.42 || z.ins.detail.Value > 0.43 {
		t.Fatalf("download progress not shown: visible=%v value=%v", z.ins.detail.Visible(), z.ins.detail.Value)
	}
}

func TestProvisionOfferedWhenWSLMissing(t *testing.T) {
	pf := &fakePF{id: "windows", ls: platform.LocalSupport{Reason: "No Ubuntu WSL2 distribution found.", CanProvision: true, ProvisionHint: "install it"}}
	z := newTest(t, pf)
	if z.tgt.provisionBtn.Visible() {
		t.Fatal("the offer belongs to the local target only, not to SSH")
	}
	z.tgt.target.SetSelected(optLocal)
	if !z.tgt.provisionBtn.Visible() {
		t.Fatal("WSL install offer must be visible when WSL/Ubuntu is missing")
	}
	z.show(pgTarget)
	z.onNext()
	if z.cur != pgTarget || !strings.Contains(z.errLabel.Text, "not possible") {
		t.Fatalf("local without WSL must block: %q", z.errLabel.Text)
	}
	pf.ls = platform.LocalSupport{Supported: true, Distros: []string{"Ubuntu"}}
	z.refreshLocal()
	if z.tgt.provisionBtn.Visible() {
		t.Fatal("offer must disappear once Ubuntu is installed")
	}
	z.onNext()
	if z.cur != pgServer {
		t.Fatalf("local install should proceed: %q", z.errLabel.Text)
	}
}

func TestMacOSHasNoLocalOption(t *testing.T) {
	z := newTest(t, &fakePF{id: "macos", ls: platform.LocalSupport{Reason: "x86-64 Linux needed"}})
	z.show(pgTarget)
	z.tgt.target.SetSelected(optLocal)
	if z.tgt.provisionBtn.Visible() {
		t.Fatal("no provisioning on macOS")
	}
	z.onNext()
	if z.cur != pgTarget {
		t.Fatal("local must be blocked on macOS")
	}
}

func TestSSHConfigAliasFillsTheForm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte("Host dune\n  HostName 203.0.113.10\n  User ranger\n  Port 2222\n  IdentityFile ~/.ssh/dune_key\n  ProxyJump bastion@198.51.100.5\n"), 0o600)
	a := test.NewApp()
	defer a.Quit()
	_, z := newWizard(a, app.New(&fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}}))

	z.tgt.host.SetText("dune")
	if z.tgt.host.Text != "203.0.113.10" || z.tgt.user.Text != "ranger" || z.tgt.port.Text != "2222" ||
		z.tgt.key.Text != "~/.ssh/dune_key" || z.tgt.jump.Text != "bastion@198.51.100.5" {
		t.Fatalf("alias not applied: host=%q user=%q port=%q key=%q jump=%q", z.tgt.host.Text, z.tgt.user.Text, z.tgt.port.Text, z.tgt.key.Text, z.tgt.jump.Text)
	}
	if !z.tgt.cfgNote.Visible() {
		t.Fatal("the user must be told where the values came from")
	}
	if z.request().Config.SSH.Jump != "bastion@198.51.100.5" {
		t.Fatal("jump host must reach the config")
	}
}

func TestEffectsSwitchStopsAnimationsAndIsRemembered(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	z.setEffects(false)
	if z.mote.box.Visible() || z.fx.on || z.effectsBtn.Text != "Effects: off" {
		t.Fatal("effects off must hide the sand and disable the page veil")
	}
	if z.a.Preferences().BoolWithFallback(prefEffects, true) {
		t.Fatal("the choice must be remembered")
	}
	z.setEffects(true)
	if !z.mote.box.Visible() || !z.fx.on {
		t.Fatal("effects on must bring everything back")
	}
}

func TestEffectsEnvOverride(t *testing.T) {
	t.Setenv("DD_INSTALLER_NO_EFFECTS", "1")
	z := newTest(t, &fakePF{id: "linux"})
	if z.effectsOn {
		t.Fatal("DD_INSTALLER_NO_EFFECTS must start the app without effects")
	}
}

func TestTokenFeedback(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	z.srv.token.SetText("abc def")
	if !strings.Contains(z.srv.tokenInfo.Text, "spaces") {
		t.Fatalf("expected a problem message: %q", z.srv.tokenInfo.Text)
	}
	z.srv.token.SetText(goodToken())
	if !strings.Contains(z.srv.tokenInfo.Text, "looks fine") {
		t.Fatalf("expected a positive message: %q", z.srv.tokenInfo.Text)
	}
}

func TestDialogsBuild(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	z.tgt.host.SetText("1.2.3.4")
	z.tgt.user.SetText("dune")
	z.showSSHHelp()
	z.showKeyCreator()
	z.showKeyCreated(sshkey.Result{PrivatePath: "/x/.ssh/k", PublicKey: "ssh-ed25519 AAAA c", Fingerprint: "SHA256:x"})
	z.showManage()
	z.runActionDialog("status", "Health check")
	if z.tgt.pass.Text != "" {
		t.Fatal("password field must stay empty")
	}
}

func TestManageNeedsConnectionFirst(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	z.showManage()
	if z.cur != pgTarget || !strings.Contains(z.errLabel.Text, "Enter the connection first") {
		t.Fatalf("manage must ask for the connection: cur=%d err=%q", z.cur, z.errLabel.Text)
	}
}

func TestFinishPageAndStartOverClearsSecrets(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	z.tgt.host.SetText("127.0.0.1")
	z.tgt.user.SetText("dune")
	z.srv.token.SetText(goodToken())
	z.tgt.pass.SetText("hunter2hunter2")
	z.ins.port = 8089
	z.showFinish()
	if z.cur != pgFinish {
		t.Fatal("finish page not shown")
	}
	// wait for the background lookup (it fails fast: nothing listens on port 22 here)
	for i := 0; i < 100 && !strings.Contains(z.fin.status.Text, "password file"); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(z.fin.ports.Text, "8089") || !strings.Contains(z.fin.url, "8089") {
		t.Fatalf("port not shown: %q %q", z.fin.ports.Text, z.fin.url)
	}
	z.fin.password = "secret-admin-pw"
	z.renderPassword()
	if strings.Contains(z.fin.pwLabel.Text, "secret-admin-pw") {
		t.Fatal("the password must be hidden until 'Show' is ticked")
	}
	z.fin.reveal.SetChecked(true)
	if z.fin.pwLabel.Text != "secret-admin-pw" {
		t.Fatal("password must be shown when asked")
	}
	z.startOver()
	if z.srv.token.Text != "" || z.tgt.pass.Text != "" || z.fin.password != "" || z.cur != pgWelcome {
		t.Fatal("start over must clear token, passwords and return to the start")
	}
}

func TestErrorLineHiddenWhenEmpty(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	z.setErr("boom")
	if !z.errLabel.Visible() {
		t.Fatal("error must be visible")
	}
	z.setErr("")
	if z.errLabel.Visible() {
		t.Fatal("empty error line must be hidden")
	}
}

// onEvent2 feeds a realistic event sequence (used by the screenshot helper).
func (z *wizard) onEvent2() {
	z.onEvent(install.Event{Type: "step", Step: "bootstrap", Status: "ok", Msg: "Download Dune Docker"})
	z.onEvent(install.Event{Type: "step", Step: "install", Status: "running", Msg: "Install Docker & start the Web UI"})
	d := assistant.Diagnosis{RuleID: "x", Title: "Port already in use", Explain: "Another service uses the chosen web port.", Risk: assistant.RiskSafe, Actions: []assistant.Action{{Description: "Use the next free Web UI port and retry."}}}
	z.onEvent(install.Event{Type: "diagnosis", Diagnosis: &d})
	z.onEvent(install.Event{Type: "log", Msg: "==> Choosing the Web UI port."})
}

// Heavy output must not freeze the window: appending is cheap, drawing is coalesced.
func TestAppendingManyLogLinesIsFast(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux"})
	start := time.Now()
	for i := 0; i < 20000; i++ {
		z.appendLog("some typical docker pull output line number " + itoa(i))
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("20000 appends took %v - drawing must be coalesced", d)
	}
}
