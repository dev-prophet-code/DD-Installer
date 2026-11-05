package ui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// installSteps are the step IDs the overall progress bar counts.
var installSteps = []string{"bootstrap", "install", "tuning", "console-setup", "start", "ready", "layout", "verify", "firewall", "post"}

const (
	maxLogLines  = 5000 // kept in memory
	shownLogRows = 800  // rendered at once
)

type installPage struct {
	summary  *widget.Label
	sumAcc   *widget.Accordion
	accept   *widget.Check
	install  *widget.Button
	cancel   *widget.Button
	progress *fyne.Container
	stepLbls map[string]*widget.Label
	stepDone map[string]bool
	overall  *widget.ProgressBar
	detail   *widget.ProgressBar
	detailL  *widget.Label
	assist   *fyne.Container
	logEntry *widget.Entry
	logFind  *widget.Entry
	autoScr  *widget.Check
	logAll   []string
	logText  string
	unsub    func()
	port     int
	// rendering the log is expensive, so appends are coalesced
	renderPending bool
}

func (z *wizard) pageInstall() fyne.CanvasObject {
	p := &z.ins
	p.stepLbls = map[string]*widget.Label{}
	p.stepDone = map[string]bool{}
	p.summary = widget.NewLabel("")
	p.summary.Wrapping = fyne.TextWrapWord
	p.accept = widget.NewCheck("I accept the disclaimer (own risk, no credentials stored, no liability)", func(bool) { z.updateInstallBtn() })
	p.install = widget.NewButton("Install now", z.onInstall)
	p.install.Importance = widget.HighImportance
	p.install.Disable()
	p.cancel = widget.NewButton("Cancel", func() { z.c.Cancel() })
	p.cancel.Hide()
	p.progress = container.NewVBox()
	p.assist = container.NewVBox()
	p.overall = widget.NewProgressBar()
	p.detail = widget.NewProgressBar()
	p.detailL = widget.NewLabel("")
	p.detail.Hide()
	p.detailL.Hide()

	p.logEntry = widget.NewMultiLineEntry()
	p.logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	p.logEntry.Wrapping = fyne.TextWrapBreak
	p.logEntry.SetMinRowsVisible(8)
	p.logEntry.SetPlaceHolder("Installation log")
	// the log is for reading and copying, not editing: undo any typing
	p.logEntry.OnChanged = func(s string) {
		if s != p.logText {
			p.logEntry.SetText(p.logText)
		}
	}
	p.logFind = widget.NewEntry()
	p.logFind.SetPlaceHolder("Filter the log (e.g. error, docker, steam)")
	p.logFind.OnChanged = func(string) { z.renderLog() }
	p.autoScr = widget.NewCheck("Follow", func(bool) { z.renderLog() })
	p.autoScr.SetChecked(true)

	p.sumAcc = widget.NewAccordion(widget.NewAccordionItem("Summary of your choices", p.summary))
	p.sumAcc.Open(0)
	head := container.NewVBox(
		heading("Begin the installation", 24),
		p.sumAcc, container.NewHBox(p.accept, p.install, p.cancel),
		p.overall, container.NewVBox(p.detailL, p.detail), widget.NewSeparator())
	left := container.NewVScroll(container.NewVBox(caption(spaced("Progress"), colSpice, 13), p.progress, p.assist))
	logHead := container.NewBorder(nil, nil, caption(spaced("Log"), colSpice, 13), p.autoScr, p.logFind)
	split := container.NewHSplit(left, container.NewBorder(logHead, nil, nil, nil, p.logEntry))
	split.SetOffset(0.42)
	return container.NewBorder(head, nil, nil, nil, split)
}

func (z *wizard) updateInstallBtn() {
	p := &z.ins
	if p.accept.Checked && !z.c.Running() {
		p.install.Enable()
	} else {
		p.install.Disable()
	}
}

func (z *wizard) updateSummary() {
	cfg := z.request().Cfg()
	var sb strings.Builder
	if cfg.Target == model.TargetSSH {
		fmt.Fprintf(&sb, "Target: %s@%s:%d", cfg.SSH.User, cfg.SSH.Host, cfg.SSH.Port)
		if cfg.SSH.Jump != "" {
			fmt.Fprintf(&sb, " via %s", cfg.SSH.Jump)
		}
		sb.WriteString("\n")
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
		fmt.Fprintf(&sb, "Deep Desert: %d instance(s)\n", cfg.DeepDesert.Instances)
	} else {
		sb.WriteString("Deep Desert: upstream default\n")
	}
	ver := install.UpstreamTag + " (tested)"
	if cfg.UpstreamLatest {
		ver = "newest upstream release"
	}
	fmt.Fprintf(&sb, "Dune Docker version: %s", ver)
	if cfg.ConfigureFirewall {
		sb.WriteString("\nFirewall: game ports will be opened (ufw/firewalld)")
	}
	z.ins.summary.SetText(sb.String())
}

func (z *wizard) onInstall() {
	p := &z.ins
	z.setErr("")
	p.progress.Objects = nil
	p.assist.Objects = nil
	p.stepLbls = map[string]*widget.Label{}
	p.stepDone = map[string]bool{}
	p.overall.SetValue(0)
	p.detail.Hide()
	p.detailL.Hide()
	p.logAll = nil
	z.renderLog()
	if err := z.c.Start(z.request()); err != nil {
		z.setErr(err.Error())
		return
	}
	p.install.Disable()
	z.back.Disable()
	p.cancel.Show()
	p.sumAcc.Close(0) // give the log the room
	z.mote.setStorm(true)
	if p.unsub != nil {
		p.unsub()
	}
	p.unsub = z.c.Subscribe(func(e install.Event) { fyne.Do(func() { z.onEvent(e) }) })
}

func (z *wizard) onEvent(e install.Event) {
	p := &z.ins
	switch e.Type {
	case "log":
		z.appendLog(e.Msg)
		if pct, ok := parseProgress(e.Msg); ok {
			p.detail.Show()
			p.detailL.Show()
			p.detail.SetValue(pct / 100)
			p.detailL.SetText("Current download / task")
		}
	case "fix":
		z.appendLog("[assistant] " + e.Msg)
	case "step":
		l, ok := p.stepLbls[e.Step]
		if !ok {
			l = widget.NewLabel("")
			p.stepLbls[e.Step] = l
			p.progress.Add(l)
		}
		icon := map[string]string{"running": "[...] ", "ok": "[ OK ] ", "failed": "[FAIL] ", "skipped": "[skip] "}[e.Status]
		l.SetText(icon + e.Msg)
		switch e.Status {
		case "ok", "skipped":
			l.Importance = widget.SuccessImportance
			p.stepDone[e.Step] = true
			p.overall.SetValue(float64(len(p.stepDone)) / float64(len(installSteps)))
			if e.Step == "console-setup" || e.Step == "install" {
				p.detail.Hide()
				p.detailL.Hide()
			}
		case "failed":
			l.Importance = widget.DangerImportance
		default:
			l.Importance = widget.MediumImportance
		}
	case "diagnosis":
		p.assist.Add(z.diagCard(*e.Diagnosis))
	case "confirm":
		d := *e.Diagnosis
		p.assist.Add(z.diagCard(d))
		z.askFix(e.ConfirmID, d)
	case "error":
		z.finishRun()
		z.setErr(e.Msg)
	case "done":
		p.overall.SetValue(1)
		p.port = e.Port
		z.finishRun()
		z.showFinish()
	}
}

func (z *wizard) finishRun() {
	z.ins.sumAcc.Open(0)
	z.ins.cancel.Hide()
	z.mote.setStorm(false)
	z.back.Enable()
	z.updateInstallBtn()
}

func (z *wizard) appendLog(s string) {
	p := &z.ins
	p.logAll = append(p.logAll, s)
	if len(p.logAll) > maxLogLines {
		p.logAll = p.logAll[len(p.logAll)-maxLogLines:]
	}
	// Re-laying out up to shownLogRows lines per received line would freeze
	// the window during busy output; draw at most every 150 ms instead.
	if !p.renderPending {
		p.renderPending = true
		time.AfterFunc(150*time.Millisecond, func() {
			fyne.Do(func() { p.renderPending = false; z.renderLog() })
		})
	}
}

// renderLog shows the (filtered) tail of the log and follows it if asked to.
func (z *wizard) renderLog() {
	p := &z.ins
	if p.logEntry == nil {
		return
	}
	lines := p.logAll
	if f := strings.ToLower(strings.TrimSpace(p.logFind.Text)); f != "" {
		var keep []string
		for _, l := range lines {
			if strings.Contains(strings.ToLower(l), f) {
				keep = append(keep, l)
			}
		}
		lines = keep
	}
	if len(lines) > shownLogRows {
		lines = lines[len(lines)-shownLogRows:]
	}
	p.logText = strings.Join(lines, "\n")
	p.logEntry.SetText(p.logText)
	if p.autoScr.Checked {
		p.logEntry.CursorRow = len(lines)
		p.logEntry.Refresh()
	}
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
