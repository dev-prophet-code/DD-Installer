// Package ui is the native desktop interface (Fyne): a real window with real
// widgets on Windows, macOS and Linux - no web server, no browser.
package ui

import (
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
)

var stepNames = []string{"Welcome", "Target", "Server", "Layout", "Scan", "Install", "Finish"}

// Page indices.
const (
	pgWelcome = iota
	pgTarget
	pgServer
	pgLayout
	pgScan
	pgInstall
	pgFinish
)

const (
	optLocal = "This computer"
	optSSH   = "Remote Linux server (SSH)"
	authKey  = "Private key"
	authPass = "Password"
	authAgnt = "ssh-agent"
)

// wizard is the window and the navigation; every page keeps its own widgets.
type wizard struct {
	c   *app.Controller
	a   fyne.App
	w   fyne.Window
	pf  platform.Platform
	cur int

	// look and effects
	steps      *stepBar
	fx         *flasher
	mote       *motes
	glint      *shimmer
	glow       *pulser
	effectsOn  bool
	effectsBtn *widget.Button

	// frame
	pages    []fyne.CanvasObject
	holder   *fyne.Container
	errLabel *widget.Label
	back     *widget.Button
	next     *widget.Button

	// pages
	tgt targetPage
	srv serverPage
	lay layoutPage
	scn scanPage
	ins installPage
	fin finishPage
}

// Run opens the installer window and blocks until it is closed.
func Run(c *app.Controller) {
	a := fyneapp.NewWithID("io.github.dev-prophet-code.dd-installer")
	a.Settings().SetTheme(duneTheme{})
	a.SetIcon(logoRes)
	w, _ := newWizard(a, c)
	w.ShowAndRun()
}

// newWizard builds the window (split out so tests can use a headless app).
func newWizard(a fyne.App, c *app.Controller) (fyne.Window, *wizard) {
	w := a.NewWindow("Dune Docker Installer " + app.Version)
	z := &wizard{c: c, a: a, w: w, pf: c.Platform()}
	z.build()
	w.Resize(fyne.NewSize(1020, 820))
	w.SetContent(z.layout())
	w.SetOnClosed(func() {
		if z.ins.unsub != nil {
			z.ins.unsub()
		}
		c.Cancel()
	})
	z.setEffects(effectsDefault(a))
	z.refreshLocal()
	z.show(pgWelcome)
	return w, z
}

func (z *wizard) build() {
	z.pages = []fyne.CanvasObject{
		z.pageWelcome(), z.pageTarget(), z.pageServer(), z.pageLayout(), z.pageScan(), z.pageInstall(), z.pageFinish(),
	}
}

func (z *wizard) layout() fyne.CanvasObject {
	bg := canvas.NewImageFromImage(renderBackdrop(1240, 1000))
	bg.FillMode = canvas.ImageFillStretch
	z.mote = newMotes(46)
	z.mote.start()
	z.fx = newFlasher()

	title := heading("DUNE DOCKER INSTALLER", 30)
	z.glow = newPulse(title, colSpice, colGold)
	sub := caption(spaced("Arrakis server setup")+"   ◆   v"+app.Version+"   ◆   "+z.pf.DisplayName(), colSandDim, 13)
	disc := widget.NewLabel(app.DisclaimerShort)
	disc.Wrapping = fyne.TextWrapWord
	more := widget.NewButton("Disclaimer", z.showDisclaimer)
	banner := panel(container.NewBorder(nil, nil, nil, more, disc))
	z.steps = newStepBar(stepNames)
	z.glint = newShimmer()

	logo := canvas.NewImageFromResource(logoRes)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(72, 72))
	z.effectsBtn = widget.NewButtonWithIcon("Effects: on", theme.VisibilityIcon(), func() { z.setEffects(!z.effectsOn) })
	head := container.NewBorder(nil, nil, logo, container.NewVBox(layout.NewSpacer(), z.effectsBtn, layout.NewSpacer()),
		container.NewVBox(layout.NewSpacer(), title, sub, layout.NewSpacer()))
	top := container.NewVBox(container.NewPadded(head), z.glint.obj, banner, container.NewPadded(z.steps.box))

	z.errLabel = widget.NewLabel("")
	z.errLabel.Wrapping = fyne.TextWrapWord
	z.errLabel.Importance = widget.DangerImportance
	z.errLabel.Hide()
	z.back = widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() { z.show(z.cur - 1) })
	z.next = widget.NewButtonWithIcon("Next", theme.NavigateNextIcon(), z.onNext)
	z.next.IconPlacement = widget.ButtonIconTrailingText
	z.next.Importance = widget.HighImportance
	bottom := panel(container.NewVBox(z.errLabel, container.NewHBox(z.back, layout.NewSpacer(), z.next)))

	z.holder = container.NewStack()
	for _, p := range z.pages {
		z.holder.Add(p)
	}
	main := container.NewBorder(top, bottom, nil, nil, panel(z.holder))
	return container.NewStack(bg, z.mote.box, container.NewPadded(main), z.fx.rect)
}

func (z *wizard) showDisclaimer() {
	l := widget.NewLabel(app.DisclaimerFull)
	l.Wrapping = fyne.TextWrapWord
	sc := container.NewVScroll(l)
	sc.SetMinSize(fyne.NewSize(620, 400))
	dialog.ShowCustom("Disclaimer", "Close", sc, z.w)
}

// setErr shows an error message, or hides the line when it is empty.
func (z *wizard) setErr(msg string) {
	z.errLabel.SetText(msg)
	if msg == "" {
		z.errLabel.Hide()
	} else {
		z.errLabel.Show()
	}
}

func (z *wizard) show(n int) {
	if n < 0 || n >= len(z.pages) {
		return
	}
	z.cur = n
	for i, p := range z.pages {
		if i == n {
			p.Show()
		} else {
			p.Hide()
		}
	}
	z.holder.Refresh() // re-run the stack layout so the newly shown page gets its size
	z.steps.set(n)
	if z.fx != nil {
		z.fx.flash()
	}
	z.setErr("")
	z.back.Disable()
	if n > pgWelcome && !z.c.Running() {
		z.back.Enable()
	}
	switch n {
	case pgInstall:
		z.next.Hide()
		z.updateSummary()
		z.updateInstallBtn()
	case pgFinish:
		z.next.Hide()
		z.back.Disable()
	default:
		z.next.Show()
		if n == pgWelcome {
			z.next.SetText("Get started")
		} else {
			z.next.SetText("Next")
		}
	}
	if n == pgLayout {
		z.estimate()
	}
}

// ---------------------------------------------------------------- request

// request collects everything the user entered.
func (z *wizard) request() app.Request {
	cfg := model.Default()
	if z.tgt.target != nil && z.tgt.target.Selected == optLocal {
		cfg.Target = model.TargetLocal
	} else {
		cfg.Target = model.TargetSSH
	}
	t := &z.tgt
	if t.host != nil {
		var port int
		fmt.Sscanf(strings.TrimSpace(t.port.Text), "%d", &port)
		method := map[string]model.AuthMethod{authKey: model.AuthKey, authPass: model.AuthPassword, authAgnt: model.AuthAgent}[t.auth.Selected]
		cfg.SSH = model.SSHConfig{Host: strings.TrimSpace(t.host.Text), Port: port, User: strings.TrimSpace(t.user.Text),
			Auth: method, KeyPath: strings.TrimSpace(t.key.Text), HostKeyFingerprint: t.hostKey, Jump: strings.TrimSpace(t.jump.Text)}
		cfg.InstallDir = strings.TrimSpace(t.installDir.Text)
		if cfg.Target == model.TargetLocal && z.pf.ID() == "windows" {
			cfg.WSLDistro = t.wslDistro.Selected
		}
	}
	if s := &z.srv; s.title != nil {
		cfg.ServerTitle = strings.TrimSpace(s.title.Text)
		cfg.Region = s.region.Selected
		fmt.Sscanf(strings.TrimSpace(s.webPort.Text), "%d", &cfg.WebPort)
		cfg.UpstreamLatest = s.latest.Checked
	}
	if l := &z.lay; l.rowsBox != nil {
		cfg.Sietches = nil
		for _, r := range l.rows {
			cfg.Sietches = append(cfg.Sietches, model.Sietch{Name: strings.TrimSpace(r.name.Text), Mode: model.Mode(r.mode.Selected)})
		}
		cfg.DeepDesert = model.DeepDesert{Instances: l.ddInstances()}
		if cfg.DeepDesert.Instances == 3 {
			cfg.DeepDesert.ThirdRole = model.Mode(l.dd3.Selected)
		}
	}
	if sc := &z.scn; sc.tuning != nil {
		cfg.ApplyTuning = sc.tuning.Checked
		cfg.ConfigureFirewall = sc.firewall.Checked
		cfg.ConsoleAllowFrom = strings.TrimSpace(sc.allowFrom.Text)
	}
	req := app.Request{Config: cfg}
	if t.pass != nil && z.srv.token != nil && z.lay.combat != nil && z.ins.accept != nil {
		req.SSHPassword, req.SSHPassphrase = t.pass.Text, t.phrase.Text
		req.FuncomToken = z.srv.token.Text
		req.ExperimentalCombat = z.lay.combat.Checked
		req.AcceptDisclaimer = z.ins.accept.Checked
	}
	return req
}

// ---------------------------------------------------------------- navigation

// fieldErrors validates the entries of the page (their inline error state is
// updated as a side effect) and returns readable messages.
func fieldErrors(fields map[string]*widget.Entry, order []string) []string {
	var out []string
	for _, label := range order {
		e := fields[label]
		if e == nil || e.Validator == nil || e.Disabled() || !e.Visible() {
			continue
		}
		if err := e.Validate(); err != nil {
			out = append(out, label+": "+err.Error())
		}
	}
	return out
}

func (z *wizard) pageErrors(step int) []string {
	var errs []string
	for _, e := range z.c.Validate(z.request()) {
		if relevant(step, e) {
			errs = append(errs, e)
		}
	}
	switch step {
	case pgTarget:
		t := &z.tgt
		if t.target.Selected == optSSH {
			errs = append(fieldErrors(map[string]*widget.Entry{"Host": t.host, "Port": t.port, "User": t.user}, []string{"Host", "Port", "User"}), errs...)
			if t.auth.Selected == authKey && strings.TrimSpace(t.key.Text) == "" {
				errs = append(errs, "Key file: choose your private key (or click Create SSH key)")
			}
		} else if ls := z.pf.LocalSupport(); !ls.Supported {
			errs = append(errs, "A local install is not possible on this computer yet: "+ls.Reason)
		}
	case pgServer:
		s := &z.srv
		errs = append(fieldErrors(map[string]*widget.Entry{"Server title": s.title, "Web console port": s.webPort}, []string{"Server title", "Web console port"}), errs...)
		if _, problem, _ := model.CheckToken(s.token.Text); problem != "" {
			errs = append(errs, "Funcom token: "+problem)
		}
	case pgLayout:
		var rows []string
		for i, r := range z.lay.rows {
			if err := r.name.Validate(); err != nil {
				rows = append(rows, fmt.Sprintf("Sietch %d: %v", i+1, err))
			}
		}
		errs = append(rows, errs...)
	case pgScan:
		if !z.scn.scanned {
			errs = append(errs, "Run the scan first (no blocking problems allowed).")
		}
		errs = append(fieldErrors(map[string]*widget.Entry{"Web console allowed from": z.scn.allowFrom}, []string{"Web console allowed from"}), errs...)
	}
	return dedupe(errs)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func (z *wizard) onNext() {
	if errs := z.pageErrors(z.cur); len(errs) > 0 {
		z.setErr(strings.Join(errs, "\n"))
		return
	}
	z.show(z.cur + 1)
}

// relevant maps validation messages to the wizard page that owns them.
func relevant(step int, e string) bool {
	owners := map[int][]string{
		pgTarget: {"ssh:", "target", "installDir", "wslDistro"},
		pgServer: {"webPort", "serverTitle", "region"},
		pgLayout: {"sietch", "at least", "at most", "deepDesert"},
		pgScan:   {"consoleAllowFrom"},
	}
	for _, p := range owners[step] {
		if strings.HasPrefix(e, p) {
			return true
		}
	}
	return false
}

func envFlag(name string) bool { return os.Getenv(name) != "" }
