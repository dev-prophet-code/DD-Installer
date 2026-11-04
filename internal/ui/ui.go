// Package ui is the native desktop interface (Fyne): a real window with real
// widgets on Windows, macOS and Linux - no web server, no browser.
package ui

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
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
	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
)

var stepNames = []string{"Target", "Server", "Layout", "Scan", "Install"}

const (
	optLocal = "This computer"
	optSSH   = "Remote Linux server (SSH)"
	authKey  = "Private key"
	authPass = "Password"
	authAgnt = "ssh-agent"
)

type sietchRow struct {
	box  *fyne.Container
	name *widget.Entry
	mode *widget.Select
}

type wizard struct {
	c   *app.Controller
	a   fyne.App
	w   fyne.Window
	pf  platform.Platform
	cur int

	steps    *stepBar
	fx       *flasher
	mote     *motes
	pages    []fyne.CanvasObject
	holder   *fyne.Container
	errLabel *widget.Label
	back     *widget.Button
	next     *widget.Button

	// page 0
	target                              *widget.RadioGroup
	localInfo                           *widget.Label
	provisionBtn                        *widget.Button
	sshBox                              *fyne.Container
	host, port, user, key, pass, phrase *widget.Entry
	auth                                *widget.Select
	keyRow, passRow, phraseRow          *widget.FormItem
	sshForm                             *widget.Form
	wslDistro                           *widget.Select
	wslRow                              *fyne.Container
	installDir                          *widget.Entry
	hostKey                             string
	// page 1
	title, webPort, token *widget.Entry
	region                *widget.Select
	// page 2
	rows     []*sietchRow
	rowsBox  *fyne.Container
	combat   *widget.Check
	dd       *widget.Select
	dd3      *widget.Select
	ddRow    *fyne.Container
	ramLabel *widget.Label
	// page 3
	scanBtn *widget.Button
	scanOut *fyne.Container
	tuning  *widget.Check
	scanned bool
	// page 4
	summary  *widget.Label
	accept   *widget.Check
	install  *widget.Button
	cancel   *widget.Button
	progress *fyne.Container
	stepLbls map[string]*widget.Label
	assist   *fyne.Container
	logEntry *widget.Entry
	logLines []string
	unsub    func()
}

// Run opens the installer window and blocks until it is closed.
func Run(c *app.Controller) {
	a := fyneapp.NewWithID("io.github.dev-prophet-code.dd-installer")
	a.Settings().SetTheme(duneTheme{})
	a.SetIcon(appIcon())
	w, _ := newWizard(a, c)
	w.ShowAndRun()
}

// newWizard builds the window (split out so tests can use a headless app).
func newWizard(a fyne.App, c *app.Controller) (fyne.Window, *wizard) {
	w := a.NewWindow("Dune Docker Installer " + app.Version)
	wz := &wizard{c: c, a: a, w: w, pf: c.Platform(), stepLbls: map[string]*widget.Label{}}
	wz.build()
	w.Resize(fyne.NewSize(1020, 820))
	w.SetContent(wz.layout())
	w.SetOnClosed(func() {
		if wz.unsub != nil {
			wz.unsub()
		}
		c.Cancel()
	})
	wz.refreshLocal()
	wz.show(0)
	return w, wz
}

func (z *wizard) layout() fyne.CanvasObject {
	// backdrop and drifting sand
	bg := canvas.NewImageFromImage(renderBackdrop(1240, 1000))
	bg.FillMode = canvas.ImageFillStretch
	z.mote = newMotes(46)
	z.mote.start()
	z.fx = newFlasher()

	// header
	title := heading("DUNE DOCKER INSTALLER", 30)
	pulse(title, colSpice, colGold)
	sub := caption(spaced("Arrakis server setup")+"   \u25C6   v"+app.Version+"   \u25C6   "+z.pf.DisplayName(), colSandDim, 13)
	disc := widget.NewLabel(app.DisclaimerShort)
	disc.Wrapping = fyne.TextWrapWord
	more := widget.NewButton("Disclaimer", z.showDisclaimer)
	banner := panel(container.NewBorder(nil, nil, nil, more, disc))
	z.steps = newStepBar(stepNames)
	top := container.NewVBox(container.NewPadded(container.NewVBox(title, sub)), newShimmer(), banner, container.NewPadded(z.steps.box))

	z.errLabel = widget.NewLabel("")
	z.errLabel.Wrapping = fyne.TextWrapWord
	z.errLabel.Importance = widget.DangerImportance
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

func (z *wizard) build() {
	z.pages = []fyne.CanvasObject{z.pageTarget(), z.pageServer(), z.pageLayout(), z.pageScan(), z.pageInstall()}
}

// ---------------------------------------------------------------- page 0

func (z *wizard) pageTarget() fyne.CanvasObject {
	z.target = widget.NewRadioGroup([]string{optLocal, optSSH}, func(string) { z.updateTarget() })
	z.localInfo = widget.NewLabel("")
	z.localInfo.Wrapping = fyne.TextWrapWord
	z.provisionBtn = widget.NewButton("Install WSL2 + Ubuntu now", z.onProvision)
	z.provisionBtn.Hide()
	z.wslDistro = widget.NewSelect(nil, nil)
	z.wslRow = container.NewVBox(widget.NewLabel("WSL2 distribution"), z.wslDistro)
	z.wslRow.Hide()

	z.host = widget.NewEntry()
	z.host.SetPlaceHolder("server.example.com or IP")
	z.port = widget.NewEntry()
	z.port.SetText("22")
	z.user = widget.NewEntry()
	z.user.SetPlaceHolder("regular user with sudo (not root)")
	z.auth = widget.NewSelect([]string{authKey, authPass, authAgnt}, func(string) { z.updateAuth() })
	z.auth.SetSelected(authKey)
	z.key = widget.NewEntry()
	z.key.SetPlaceHolder("~/.ssh/id_ed25519")
	if ks := z.pf.DefaultSSHKeys(); len(ks) > 0 {
		z.key.SetText(ks[0])
	}
	browse := widget.NewButton("Browse...", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err == nil && r != nil {
				z.key.SetText(r.URI().Path())
				_ = r.Close()
			}
		}, z.w)
		d.Show()
	})
	z.phrase = widget.NewPasswordEntry()
	z.pass = widget.NewPasswordEntry()
	z.keyRow = widget.NewFormItem("Key file", container.NewBorder(nil, nil, nil, browse, z.key))
	z.phraseRow = widget.NewFormItem("Key passphrase", z.phrase)
	z.passRow = widget.NewFormItem("Password (also for sudo)", z.pass)
	z.sshForm = widget.NewForm(
		widget.NewFormItem("Host", z.host), widget.NewFormItem("Port", z.port), widget.NewFormItem("User", z.user),
		widget.NewFormItem("Authentication", z.auth), z.keyRow, z.phraseRow, z.passRow)
	helpBtn := widget.NewButtonWithIcon("SSH help", theme.HelpIcon(), z.showSSHHelp)
	keyBtn := widget.NewButtonWithIcon("Create SSH key", theme.ContentAddIcon(), z.showKeyCreator)
	z.sshBox = container.NewVBox(z.sshForm, container.NewHBox(keyBtn, helpBtn))
	z.installDir = widget.NewEntry()
	z.installDir.SetPlaceHolder("blank = ~/dune-awakening-selfhost-docker")

	z.updateAuth()
	z.target.SetSelected(optSSH)
	return container.NewVScroll(container.NewVBox(
		heading("Where should it be installed?", 24),
		z.target, z.localInfo, z.provisionBtn, z.wslRow, z.sshBox,
		widget.NewForm(widget.NewFormItem("Install folder", z.installDir)),
	))
}

func (z *wizard) updateAuth() {
	if z.sshForm == nil {
		return // called while the page is still being built
	}
	a := z.auth.Selected
	z.sshForm.Items = []*widget.FormItem{z.sshForm.Items[0], z.sshForm.Items[1], z.sshForm.Items[2], z.sshForm.Items[3]}
	switch a {
	case authKey:
		z.sshForm.Items = append(z.sshForm.Items, z.keyRow, z.phraseRow)
	case authPass:
		z.sshForm.Items = append(z.sshForm.Items, z.passRow)
	}
	z.sshForm.Refresh()
	z.scanned = false
}

func (z *wizard) updateTarget() {
	if z.sshBox == nil || z.wslRow == nil {
		return
	}
	local := z.target.Selected == optLocal
	if local {
		z.sshBox.Hide()
		z.wslRow.Hide()
		if z.pf.ID() == "windows" && len(z.pf.LocalSupport().Distros) > 0 {
			z.wslRow.Show()
		}
	} else {
		z.sshBox.Show()
		z.wslRow.Hide()
	}
	z.scanned = false
	z.refreshLocal()
}

// refreshLocal re-reads whether a local install is possible and updates the hints.
func (z *wizard) refreshLocal() {
	if z.localInfo == nil {
		return
	}
	ls := z.pf.LocalSupport()
	z.localInfo.SetText(ls.Reason)
	if len(ls.Distros) > 0 {
		z.wslDistro.SetOptions(ls.Distros)
		if z.wslDistro.Selected == "" {
			z.wslDistro.SetSelected(ls.Distros[0])
		}
	}
	if !ls.Supported && ls.CanProvision {
		z.provisionBtn.Show()
	} else {
		z.provisionBtn.Hide()
	}
	if z.target.Selected == optLocal && ls.Supported && z.pf.ID() == "windows" {
		z.wslRow.Show()
	}
}

// onProvision offers to install the missing WSL2 distribution and continues.
func (z *wizard) onProvision() {
	hint := z.pf.LocalSupport().ProvisionHint
	l := widget.NewLabel(hint)
	l.Wrapping = fyne.TextWrapWord
	sc := container.NewVScroll(l)
	sc.SetMinSize(fyne.NewSize(480, 140))
	dialog.ShowCustomConfirm("Install WSL2 and Ubuntu?", "Install", "Cancel", sc, func(ok bool) {
		if !ok {
			return
		}
		logL := widget.NewLabel("Starting...")
		logL.Wrapping = fyne.TextWrapWord
		bar := widget.NewProgressBarInfinite()
		prog := dialog.NewCustomWithoutButtons("Installing WSL2 + Ubuntu", container.NewVBox(bar, logL), z.w)
		prog.Show()
		go func() {
			reboot, err := z.c.ProvisionLocal(context.Background(), func(s string) {
				fyne.Do(func() { logL.SetText(s) })
			})
			fyne.Do(func() {
				prog.Hide()
				switch {
				case err != nil:
					dialog.ShowError(err, z.w)
				case reboot:
					dialog.ShowInformation("Restart required",
						"Windows must be restarted to finish installing WSL2.\n\nRestart the computer and start DD-Installer again. The setup continues with the Ubuntu that is now installed.", z.w)
				default:
					z.refreshLocal()
					z.target.SetSelected(optLocal)
					dialog.ShowInformation("Ubuntu is ready", "WSL2 and Ubuntu are installed. Press Next to continue the setup.", z.w)
				}
			})
		}()
	}, z.w)
}

// ---------------------------------------------------------------- page 1

func (z *wizard) pageServer() fyne.CanvasObject {
	z.title = widget.NewEntry()
	z.title.SetText("My Dune Server")
	z.region = widget.NewSelect(model.Regions, nil)
	z.region.SetSelected("Europe")
	z.webPort = widget.NewEntry()
	z.webPort.SetText("8088")
	z.token = widget.NewPasswordEntry()
	z.token.SetPlaceHolder("Funcom self-host service token")
	tokNote := widget.NewLabel("The token is kept in memory only and sent through the encrypted channel to your server.")
	tokNote.Wrapping = fyne.TextWrapWord
	return container.NewVScroll(container.NewVBox(
		heading("Your server", 24),
		widget.NewForm(
			widget.NewFormItem("Server title", z.title),
			widget.NewFormItem("Region", z.region),
			widget.NewFormItem("Web console port", z.webPort),
			widget.NewFormItem("Funcom token", z.token)),
		tokNote))
}

// ---------------------------------------------------------------- page 2

func (z *wizard) pageLayout() fyne.CanvasObject {
	z.rowsBox = container.NewVBox()
	add := widget.NewButton("+ Add Sietch", func() { z.addSietch("", "PVE") })
	z.combat = widget.NewCheck("Also apply PvP/PvE per Sietch (experimental - verify in game)", nil)
	z.dd = widget.NewSelect([]string{"Upstream default", "1", "2 (one PvE + one PvP)", "3"}, func(string) { z.updateDD() })
	z.dd.SetSelected("Upstream default")
	z.dd3 = widget.NewSelect([]string{"PVE", "PVP"}, func(string) {})
	z.dd3.SetSelected("PVE")
	z.ddRow = container.NewVBox(widget.NewLabel("Third instance ruleset"), z.dd3)
	z.ddRow.Hide()
	note := widget.NewLabel("Upstream keeps the first instance's existing ruleset, gives the second the opposite one and lets you choose only the third.")
	note.Wrapping = fyne.TextWrapWord
	z.ramLabel = widget.NewLabel("")
	z.ramLabel.Wrapping = fyne.TextWrapWord
	z.ramLabel.Importance = widget.WarningImportance
	z.addSietch("Sietch 1", "PVE")
	return container.NewVScroll(container.NewVBox(
		heading("Sietches & Deep Desert", 24),
		caption(spaced("Hagga Basin sietches - name and ruleset"), colSpice, 13),
		z.rowsBox, add, z.combat,
		widget.NewSeparator(),
		caption(spaced("Deep Desert"), colSpice, 13),
		widget.NewForm(widget.NewFormItem("Instances", z.dd)), z.ddRow, note, z.ramLabel))
}

func (z *wizard) addSietch(name, mode string) {
	if name == "" {
		name = fmt.Sprintf("Sietch %d", len(z.rows)+1)
	}
	r := &sietchRow{name: widget.NewEntry(), mode: widget.NewSelect([]string{"PVE", "PVP"}, func(string) {})}
	r.name.SetText(name)
	r.name.OnChanged = func(string) { z.estimate() }
	r.mode.SetSelected(mode)
	del := widget.NewButton("Remove", nil)
	r.box = container.NewBorder(nil, nil, nil, container.NewHBox(r.mode, del), r.name)
	del.OnTapped = func() {
		if len(z.rows) <= 1 {
			return
		}
		for i, x := range z.rows {
			if x == r {
				z.rows = append(z.rows[:i], z.rows[i+1:]...)
				break
			}
		}
		z.rowsBox.Remove(r.box)
		z.estimate()
	}
	z.rows = append(z.rows, r)
	z.rowsBox.Add(r.box)
	z.estimate()
}

func (z *wizard) updateDD() {
	if z.ddRow == nil {
		return
	}
	if strings.HasPrefix(z.dd.Selected, "3") {
		z.ddRow.Show()
	} else {
		z.ddRow.Hide()
	}
	z.estimate()
}

func (z *wizard) ddInstances() int {
	s := z.dd.Selected
	if s == "" || strings.HasPrefix(s, "Upstream") {
		return 0
	}
	n, _ := strconv.Atoi(s[:1])
	return n
}

func (z *wizard) estimate() {
	if z.ramLabel == nil {
		return
	}
	cfg := z.request().Cfg()
	z.ramLabel.SetText(fmt.Sprintf("Estimated RAM needed for this layout: about %d GB (heuristic - the scan checks your server).", sizing.RequiredRAM(cfg)))
}

// ---------------------------------------------------------------- page 3

func (z *wizard) pageScan() fyne.CanvasObject {
	z.scanBtn = widget.NewButton("Scan target", z.onScan)
	z.scanBtn.Importance = widget.HighImportance
	z.scanOut = container.NewVBox()
	z.tuning = widget.NewCheck("Apply recommended tuning (PostgreSQL, RabbitMQ, swap)", func(bool) {})
	z.tuning.SetChecked(true)
	return container.NewVScroll(container.NewVBox(
		heading("Scan the target", 24),
		z.scanBtn, z.scanOut, z.tuning))
}

func (z *wizard) onScan() {
	z.setErr("")
	z.scanBtn.Disable()
	z.scanOut.Objects = []fyne.CanvasObject{widget.NewLabel("Scanning...")}
	z.scanOut.Refresh()
	req := z.request()
	go func() {
		res := z.c.Scan(context.Background(), req)
		fyne.Do(func() { z.scanBtn.Enable(); z.showScan(res) })
	}()
}

func (z *wizard) showScan(r app.ScanResult) {
	z.scanOut.Objects = nil
	add := func(o fyne.CanvasObject) { z.scanOut.Add(o) }
	if r.HostKeyRequired {
		z.scanOut.Refresh()
		if r.KeyMismatch {
			dialog.ShowError(fmt.Errorf("the SSH host key CHANGED (%s).\nThis can mean a man-in-the-middle attack or a reinstalled server. Verify with the server owner before continuing", r.Fingerprint), z.w)
			return
		}
		msg := widget.NewLabel("First connection to this server.\n\nSSH host key fingerprint:\n" + r.Fingerprint +
			"\n\nCompare it with `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the server before trusting it.")
		msg.Wrapping = fyne.TextWrapWord
		dialog.ShowCustomConfirm("Trust this SSH host key?", "Trust", "Cancel", msg, func(ok bool) {
			if ok {
				z.hostKey = r.Fingerprint
				z.onScan()
			}
		}, z.w)
		return
	}
	if r.Err != "" {
		l := widget.NewLabel("Error: " + r.Err)
		l.Wrapping = fyne.TextWrapWord
		l.Importance = widget.DangerImportance
		add(l)
		for _, d := range r.Diagnoses {
			add(z.diagCard(d))
		}
		z.scanOut.Refresh()
		z.scanned = false
		return
	}
	h, rep := r.Host, r.Report
	hd := widget.NewLabel(fmt.Sprintf("Target: %s\n%s, %s\n%d threads, %.1f GB RAM, %.0f GB free, AVX2: %v\nLayout needs about %d GB; this host fits up to %d Sietch(es) without Deep Desert.",
		r.Target, firstNonEmpty(h.Distro, h.OS), h.CPUModel, h.CPUThreads, h.MemTotalGiB(), h.DiskFreeGiB(), h.AVX2, rep.RequiredRAMGiB, rep.MaxSietches))
	hd.Wrapping = fyne.TextWrapWord
	add(hd)
	for _, f := range rep.Findings {
		l := widget.NewLabel(strings.ToUpper(string(f.Level)) + ": " + f.Message)
		l.Wrapping = fyne.TextWrapWord
		switch f.Level {
		case sizing.Error:
			l.Importance = widget.DangerImportance
		case sizing.Warn:
			l.Importance = widget.WarningImportance
		}
		add(l)
	}
	if rep.WSLConfig != "" {
		e := widget.NewMultiLineEntry()
		e.SetText("# %UserProfile%\\.wslconfig\n" + rep.WSLConfig)
		e.SetMinRowsVisible(4)
		add(e)
	}
	z.scanned = rep.OK
	if !rep.OK {
		l := widget.NewLabel("Blocking problems found - fix them or choose another target.")
		l.Importance = widget.DangerImportance
		add(l)
	}
	z.scanOut.Refresh()
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------------- page 4

func (z *wizard) pageInstall() fyne.CanvasObject {
	z.summary = widget.NewLabel("")
	z.summary.Wrapping = fyne.TextWrapWord
	z.accept = widget.NewCheck("I accept the disclaimer (own risk, no credentials stored, no liability)", func(bool) { z.updateInstallBtn() })
	z.install = widget.NewButton("Install now", z.onInstall)
	z.install.Importance = widget.HighImportance
	z.install.Disable()
	z.cancel = widget.NewButton("Cancel", func() { z.c.Cancel() })
	z.cancel.Hide()
	z.progress = container.NewVBox()
	z.assist = container.NewVBox()
	z.logEntry = widget.NewMultiLineEntry()
	z.logEntry.Wrapping = fyne.TextWrapBreak
	z.logEntry.SetMinRowsVisible(14)
	return container.NewVScroll(container.NewVBox(
		heading("Begin the installation", 24),
		z.summary, z.accept, container.NewHBox(z.install, z.cancel), z.progress, z.assist, z.logEntry))
}

func (z *wizard) updateInstallBtn() {
	if z.accept.Checked && !z.c.Running() {
		z.install.Enable()
	} else {
		z.install.Disable()
	}
}

func (z *wizard) updateSummary() {
	cfg := z.request().Cfg()
	var sb strings.Builder
	if cfg.Target == model.TargetSSH {
		fmt.Fprintf(&sb, "Target: %s@%s:%d\n", cfg.SSH.User, cfg.SSH.Host, cfg.SSH.Port)
	} else {
		sb.WriteString("Target: this computer\n")
	}
	fmt.Fprintf(&sb, "Server: %s (%s), web console port %d\n", cfg.ServerTitle, cfg.Region, cfg.WebPort)
	var names []string
	for _, s := range cfg.Sietches {
		names = append(names, s.Name+" ["+string(s.Mode)+"]")
	}
	fmt.Fprintf(&sb, "Sietches: %s\n", strings.Join(names, ", "))
	if cfg.DeepDesert.Instances > 0 {
		fmt.Fprintf(&sb, "Deep Desert: %d instance(s)", cfg.DeepDesert.Instances)
	} else {
		sb.WriteString("Deep Desert: upstream default")
	}
	z.summary.SetText(sb.String())
}

func (z *wizard) onInstall() {
	z.setErr("")
	z.progress.Objects = nil
	z.assist.Objects = nil
	z.stepLbls = map[string]*widget.Label{}
	z.logEntry.SetText("")
	z.logLines = nil
	if err := z.c.Start(z.request()); err != nil {
		z.setErr(err.Error())
		return
	}
	z.install.Disable()
	z.back.Disable()
	z.cancel.Show()
	z.mote.setStorm(true)
	if z.unsub != nil {
		z.unsub()
	}
	z.unsub = z.c.Subscribe(func(e install.Event) { fyne.Do(func() { z.onEvent(e) }) })
}

func (z *wizard) onEvent(e install.Event) {
	switch e.Type {
	case "log":
		z.appendLog(e.Msg)
	case "fix":
		z.appendLog("[assistant] " + e.Msg)
	case "step":
		l, ok := z.stepLbls[e.Step]
		if !ok {
			l = widget.NewLabel("")
			z.stepLbls[e.Step] = l
			z.progress.Add(l)
		}
		icon := map[string]string{"running": "[...] ", "ok": "[ OK ] ", "failed": "[FAIL] ", "skipped": "[skip] "}[e.Status]
		l.SetText(icon + e.Msg)
		switch e.Status {
		case "ok":
			l.Importance = widget.SuccessImportance
		case "failed":
			l.Importance = widget.DangerImportance
		default:
			l.Importance = widget.MediumImportance
		}
	case "diagnosis":
		z.assist.Add(z.diagCard(*e.Diagnosis))
	case "confirm":
		d := *e.Diagnosis
		z.assist.Add(z.diagCard(d))
		z.askFix(e.ConfirmID, d)
	case "error":
		z.finish()
		z.setErr(e.Msg)
	case "done":
		z.finish()
		dialog.ShowInformation("Installation finished",
			fmt.Sprintf("Open the web console on port %d of your server and sign in with the admin password stored in runtime/secrets/admin-web-password.txt on the server.\n\nOpen the game ports from the documentation in your firewall/router.", z.request().Config.WebPort), z.w)
	}
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

func (z *wizard) finish() {
	z.mote.setStorm(false)
	z.cancel.Hide()
	z.back.Enable()
	z.updateInstallBtn()
}

func (z *wizard) appendLog(s string) {
	z.logLines = append(z.logLines, s)
	if len(z.logLines) > 600 {
		z.logLines = z.logLines[len(z.logLines)-600:]
	}
	z.logEntry.SetText(strings.Join(z.logLines, "\n"))
	z.logEntry.CursorRow = len(z.logLines)
	z.logEntry.Refresh()
}

func (z *wizard) diagCard(d assistant.Diagnosis) fyne.CanvasObject {
	body := widget.NewLabel(d.Explain)
	body.Wrapping = fyne.TextWrapWord
	items := []fyne.CanvasObject{body}
	for _, a := range d.Actions {
		l := widget.NewLabel("- " + a.Description)
		l.Wrapping = fyne.TextWrapWord
		items = append(items, l)
	}
	if u, err := url.Parse(d.DocURL); err == nil && d.DocURL != "" {
		items = append(items, widget.NewHyperlink("Open the documentation", u))
	}
	return widget.NewCard(d.Title, "Fix type: "+string(d.Risk), container.NewVBox(items...))
}

func (z *wizard) askFix(id string, d assistant.Diagnosis) {
	var acts []string
	for _, a := range d.Actions {
		acts = append(acts, "- "+a.Description)
	}
	l := widget.NewLabel(d.Explain + "\n\nThe assistant would:\n" + strings.Join(acts, "\n"))
	l.Wrapping = fyne.TextWrapWord
	sc := container.NewVScroll(l)
	sc.SetMinSize(fyne.NewSize(520, 220))
	dialog.ShowCustomConfirm("Apply this fix? - "+d.Title, "Apply fix", "Decline", sc, func(ok bool) { z.c.Confirm(id, ok) }, z.w)
}

// ---------------------------------------------------------------- shared

func (z *wizard) request() app.Request {
	cfg := model.Default()
	if z.target != nil && z.target.Selected == optLocal {
		cfg.Target = model.TargetLocal
	} else {
		cfg.Target = model.TargetSSH
	}
	if z.host != nil {
		p, _ := strconv.Atoi(strings.TrimSpace(z.port.Text))
		method := map[string]model.AuthMethod{authKey: model.AuthKey, authPass: model.AuthPassword, authAgnt: model.AuthAgent}[z.auth.Selected]
		cfg.SSH = model.SSHConfig{Host: strings.TrimSpace(z.host.Text), Port: p, User: strings.TrimSpace(z.user.Text),
			Auth: method, KeyPath: strings.TrimSpace(z.key.Text), HostKeyFingerprint: z.hostKey}
		cfg.InstallDir = strings.TrimSpace(z.installDir.Text)
		if cfg.Target == model.TargetLocal && z.pf.ID() == "windows" {
			cfg.WSLDistro = z.wslDistro.Selected
		}
	}
	if z.title != nil {
		cfg.ServerTitle = strings.TrimSpace(z.title.Text)
		cfg.Region = z.region.Selected
		cfg.WebPort, _ = strconv.Atoi(strings.TrimSpace(z.webPort.Text))
	}
	if z.rowsBox != nil {
		cfg.Sietches = nil
		for _, r := range z.rows {
			cfg.Sietches = append(cfg.Sietches, model.Sietch{Name: strings.TrimSpace(r.name.Text), Mode: model.Mode(r.mode.Selected)})
		}
		cfg.DeepDesert = model.DeepDesert{Instances: z.ddInstances()}
		if cfg.DeepDesert.Instances == 3 {
			cfg.DeepDesert.ThirdRole = model.Mode(z.dd3.Selected)
		}
	}
	if z.tuning != nil {
		cfg.ApplyTuning = z.tuning.Checked
	}
	req := app.Request{Config: cfg}
	if z.pass != nil && z.token != nil && z.combat != nil && z.accept != nil {
		req.SSHPassword, req.SSHPassphrase = z.pass.Text, z.phrase.Text
		req.FuncomToken = z.token.Text
		req.ExperimentalCombat = z.combat.Checked
		req.AcceptDisclaimer = z.accept.Checked
	}
	return req
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
	if n > 0 && !z.c.Running() {
		z.back.Enable()
	}
	if n == len(z.pages)-1 {
		z.next.Hide()
		z.updateSummary()
		z.updateInstallBtn()
	} else {
		z.next.Show()
	}
	if n == 2 {
		z.estimate()
	}
}

func (z *wizard) onNext() {
	var errs []string
	for _, e := range z.c.Validate(z.request()) {
		if relevant(z.cur, e) {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 && z.cur == 1 && strings.TrimSpace(z.token.Text) == "" {
		errs = append(errs, "Enter your Funcom token.")
	}
	if len(errs) == 0 && z.cur == 3 && !z.scanned {
		errs = append(errs, "Run the scan first (no blocking problems allowed).")
	}
	if z.cur == 0 && z.target.Selected == optLocal && !z.pf.LocalSupport().Supported {
		errs = append(errs, "A local install is not possible on this computer yet: "+z.pf.LocalSupport().Reason)
	}
	if len(errs) > 0 {
		z.setErr(strings.Join(errs, "\n"))
		return
	}
	z.show(z.cur + 1)
}

// relevant maps validation messages to the wizard page that owns them.
func relevant(step int, e string) bool {
	owners := map[int][]string{
		0: {"ssh:", "target", "installDir", "wslDistro"},
		1: {"webPort", "serverTitle", "region"},
		2: {"sietch", "at least", "at most", "deepDesert"},
	}
	for _, p := range owners[step] {
		if strings.HasPrefix(e, p) {
			return true
		}
	}
	return false
}
