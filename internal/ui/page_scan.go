package ui

import (
	"context"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
)

type scanPage struct {
	btn       *widget.Button
	out       *fyne.Container
	tuning    *widget.Check
	firewall  *widget.Check
	allowFrom *widget.Entry
	fwBox     *fyne.Container
	scanned   bool
}

func (z *wizard) pageScan() fyne.CanvasObject {
	s := &z.scn
	s.btn = widget.NewButton("Scan target", z.onScan)
	s.btn.Importance = widget.HighImportance
	s.out = container.NewVBox()
	s.tuning = widget.NewCheck("Apply recommended tuning (PostgreSQL, RabbitMQ, swap)", nil)
	s.tuning.SetChecked(true)
	s.firewall = widget.NewCheck("Open the game ports in the server firewall (ufw / firewalld)", func(bool) { z.updateScanVisibility() })
	s.allowFrom = widget.NewEntry()
	s.allowFrom.SetPlaceHolder("blank = anyone; or your IP / network, e.g. 203.0.113.7 or 203.0.113.0/24")
	s.allowFrom.Validator = model.ValidAllowFrom
	fwNote := widget.NewLabel("Only adds allow rules to a firewall that is already active; it never enables, disables or resets one, and SSH stays open. Cloud firewalls and routers must be set up by you (see the router & firewall guide).")
	fwNote.Wrapping = fyne.TextWrapWord
	s.fwBox = container.NewVBox(fwNote, widget.NewForm(widget.NewFormItem("Web console allowed from", s.allowFrom)))
	s.fwBox.Hide()
	z.updateScanVisibility()
	return container.NewVScroll(container.NewVBox(
		heading("Scan the target", 24),
		s.btn, s.out, widget.NewSeparator(), s.tuning, s.firewall, s.fwBox))
}

// updateScanVisibility shows the firewall options only for remote servers.
func (z *wizard) updateScanVisibility() {
	s := &z.scn
	if s.firewall == nil || z.tgt.target == nil {
		return
	}
	remote := z.tgt.target.Selected == optSSH
	if remote {
		s.firewall.Show()
	} else {
		s.firewall.Hide()
		s.firewall.SetChecked(false)
	}
	if remote && s.firewall.Checked {
		s.fwBox.Show()
	} else if s.fwBox != nil {
		s.fwBox.Hide()
	}
}

func (z *wizard) onScan() {
	s := &z.scn
	z.setErr("")
	s.btn.Disable()
	s.out.Objects = []fyne.CanvasObject{widget.NewLabel("Scanning...")}
	s.out.Refresh()
	req := z.request()
	go func() {
		res := z.c.Scan(context.Background(), req)
		fyne.Do(func() { s.btn.Enable(); z.showScan(res) })
	}()
}

func (z *wizard) showScan(r app.ScanResult) {
	s := &z.scn
	s.out.Objects = nil
	add := func(o fyne.CanvasObject) { s.out.Add(o) }
	if r.HostKeyRequired {
		s.out.Refresh()
		if r.KeyMismatch {
			dialog.ShowError(fmt.Errorf("the SSH host key CHANGED (%s).\nThis can mean a man-in-the-middle attack or a reinstalled server. Verify with the server owner before continuing", r.Fingerprint), z.w)
			return
		}
		msg := widget.NewLabel("First connection to this server.\n\nSSH host key fingerprint:\n" + r.Fingerprint +
			"\n\nCompare it with `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the server before trusting it.")
		msg.Wrapping = fyne.TextWrapWord
		dialog.ShowCustomConfirm("Trust this SSH host key?", "Trust", "Cancel", msg, func(ok bool) {
			if ok {
				z.tgt.hostKey = r.Fingerprint
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
		s.out.Refresh()
		s.scanned = false
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
	s.scanned = rep.OK
	if !rep.OK {
		l := widget.NewLabel("Blocking problems found - fix them or choose another target.")
		l.Importance = widget.DangerImportance
		add(l)
	}
	s.out.Refresh()
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}
