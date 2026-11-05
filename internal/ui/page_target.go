package ui

import (
	"context"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/sshconfig"
)

type targetPage struct {
	target       *widget.RadioGroup
	localInfo    *widget.Label
	provisionBtn *widget.Button
	sshBox       *fyne.Container
	sshForm      *widget.Form
	host, port   *widget.Entry
	user, key    *widget.Entry
	pass, phrase *widget.Entry
	jump         *widget.Entry
	installDir   *widget.Entry
	auth         *widget.Select
	keyRow       *widget.FormItem
	phraseRow    *widget.FormItem
	passRow      *widget.FormItem
	jumpRow      *widget.FormItem
	baseRows     []*widget.FormItem
	wslDistro    *widget.Select
	wslRow       *fyne.Container
	cfgNote      *widget.Label
	cfg          *sshconfig.Config
	hostKey      string // fingerprint the user confirmed in this session
}

func (z *wizard) pageTarget() fyne.CanvasObject {
	t := &z.tgt
	t.cfg, _ = sshconfig.Load()
	t.target = widget.NewRadioGroup([]string{optLocal, optSSH}, func(string) { z.updateTarget() })
	t.localInfo = widget.NewLabel("")
	t.localInfo.Wrapping = fyne.TextWrapWord
	t.provisionBtn = widget.NewButton("Install WSL2 + Ubuntu now", z.onProvision)
	t.provisionBtn.Hide()
	t.wslDistro = widget.NewSelect(nil, nil)
	t.wslRow = container.NewVBox(widget.NewLabel("WSL2 distribution"), t.wslDistro)
	t.wslRow.Hide()

	t.host = widget.NewEntry()
	t.host.SetPlaceHolder("server.example.com, IP or a name from ~/.ssh/config")
	t.host.Validator = model.ValidHost
	t.host.OnChanged = func(s string) { t.hostKey = ""; z.scn.scanned = false; z.applySSHConfig(s) }
	t.port = widget.NewEntry()
	t.port.SetText("22")
	t.port.Validator = model.ValidPort
	t.user = widget.NewEntry()
	t.user.SetPlaceHolder("regular user with sudo (not root)")
	t.user.Validator = model.ValidUser
	t.auth = widget.NewSelect([]string{authKey, authPass, authAgnt}, func(string) { z.updateAuth() })
	t.key = widget.NewEntry()
	t.key.SetPlaceHolder("~/.ssh/id_ed25519")
	if ks := z.pf.DefaultSSHKeys(); len(ks) > 0 {
		t.key.SetText(ks[0])
	}
	browse := widget.NewButton("Browse...", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err == nil && r != nil {
				t.key.SetText(r.URI().Path())
				_ = r.Close()
			}
		}, z.w)
		d.Show()
	})
	t.phrase = widget.NewPasswordEntry()
	t.pass = widget.NewPasswordEntry()
	t.jump = widget.NewEntry()
	t.jump.SetPlaceHolder("optional: user@jump-host[:port] (must be in ~/.ssh/known_hosts)")
	t.keyRow = widget.NewFormItem("Key file", container.NewBorder(nil, nil, nil, browse, t.key))
	t.phraseRow = widget.NewFormItem("Key passphrase", t.phrase)
	t.passRow = widget.NewFormItem("Password (also for sudo)", t.pass)
	t.jumpRow = widget.NewFormItem("Jump host", t.jump)
	t.baseRows = []*widget.FormItem{widget.NewFormItem("Host", t.host), widget.NewFormItem("Port", t.port),
		widget.NewFormItem("User", t.user), widget.NewFormItem("Authentication", t.auth)}
	t.sshForm = widget.NewForm()
	t.cfgNote = widget.NewLabel("")
	t.cfgNote.Wrapping = fyne.TextWrapWord
	t.cfgNote.Hide()

	t.installDir = widget.NewEntry()
	t.installDir.SetPlaceHolder("blank = ~/dune-awakening-selfhost-docker")

	keyBtn := widget.NewButtonWithIcon("Create SSH key", theme.ContentAddIcon(), z.showKeyCreator)
	helpBtn := widget.NewButtonWithIcon("SSH help", theme.HelpIcon(), z.showSSHHelp)
	cfgBtn := widget.NewButtonWithIcon("From ~/.ssh/config", theme.DownloadIcon(), z.showSSHConfigPicker)
	if len(t.cfg.Aliases()) == 0 {
		cfgBtn.Hide()
	}
	manageBtn := widget.NewButtonWithIcon("Manage existing server...", theme.SettingsIcon(), z.showManage)
	t.sshBox = container.NewVBox(t.sshForm, t.cfgNote, container.NewHBox(keyBtn, cfgBtn, helpBtn))

	t.auth.SetSelected(authKey)
	t.target.SetSelected(optSSH)
	return container.NewVScroll(container.NewVBox(
		heading("Where should it be installed?", 24),
		t.target, t.localInfo, t.provisionBtn, t.wslRow, t.sshBox,
		widget.NewForm(widget.NewFormItem("Install folder", t.installDir)),
		widget.NewSeparator(),
		container.NewHBox(manageBtn, widget.NewLabel("update, repair, back up or remove a server that is already installed")),
	))
}

// applySSHConfig fills the form when the host field holds a name from ~/.ssh/config.
func (z *wizard) applySSHConfig(alias string) {
	t := &z.tgt
	if t.cfg == nil {
		return
	}
	e, found := t.cfg.Lookup(strings.TrimSpace(alias))
	if !found {
		t.cfgNote.Hide()
		return
	}
	if e.HostName != "" && e.HostName != alias {
		t.host.SetText(e.HostName) // re-enters OnChanged with the real host name
	}
	if e.Port > 0 {
		t.port.SetText(strconv.Itoa(e.Port))
	}
	if e.User != "" {
		t.user.SetText(e.User)
	}
	if e.IdentityFile != "" {
		t.key.SetText(e.IdentityFile)
		t.auth.SetSelected(authKey)
	}
	if e.ProxyJump != "" {
		t.jump.SetText(e.ProxyJump)
	}
	t.cfgNote.SetText("Loaded the settings of '" + alias + "' from ~/.ssh/config.")
	t.cfgNote.Show()
}

func (z *wizard) showSSHConfigPicker() {
	t := &z.tgt
	sel := widget.NewSelect(t.cfg.Aliases(), nil)
	dialog.ShowCustomConfirm("Use a host from ~/.ssh/config", "Use", "Cancel", container.NewVBox(widget.NewLabel("Pick one of your saved SSH hosts:"), sel), func(ok bool) {
		if ok && sel.Selected != "" {
			t.host.SetText(sel.Selected)
			z.applySSHConfig(sel.Selected)
		}
	}, z.w)
}

func (z *wizard) updateAuth() {
	t := &z.tgt
	if t.sshForm == nil {
		return
	}
	items := append([]*widget.FormItem(nil), t.baseRows...)
	switch t.auth.Selected {
	case authKey:
		items = append(items, t.keyRow, t.phraseRow)
	case authPass:
		items = append(items, t.passRow)
	}
	items = append(items, t.jumpRow)
	t.sshForm.Items = items
	t.sshForm.Refresh()
	z.scn.scanned = false
}

func (z *wizard) updateTarget() {
	t := &z.tgt
	if t.sshBox == nil || t.wslRow == nil {
		return
	}
	if t.target.Selected == optLocal {
		t.sshBox.Hide()
		t.wslRow.Hide()
		if z.pf.ID() == "windows" && len(z.pf.LocalSupport().Distros) > 0 {
			t.wslRow.Show()
		}
	} else {
		t.sshBox.Show()
		t.wslRow.Hide()
	}
	z.scn.scanned = false
	z.refreshLocal()
	z.updateScanVisibility()
}

// refreshLocal re-reads whether a local install is possible and updates the hints.
func (z *wizard) refreshLocal() {
	t := &z.tgt
	if t.localInfo == nil {
		return
	}
	ls := z.pf.LocalSupport()
	t.localInfo.SetText(ls.Reason)
	if len(ls.Distros) > 0 {
		t.wslDistro.SetOptions(ls.Distros)
		if t.wslDistro.Selected == "" {
			t.wslDistro.SetSelected(ls.Distros[0])
		}
	}
	if !ls.Supported && ls.CanProvision && t.target.Selected == optLocal {
		t.provisionBtn.Show()
	} else {
		t.provisionBtn.Hide()
	}
	if t.target.Selected == optLocal && ls.Supported && z.pf.ID() == "windows" {
		t.wslRow.Show()
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
					z.tgt.target.SetSelected(optLocal)
					dialog.ShowInformation("Ubuntu is ready", "WSL2 and Ubuntu are installed. Press Next to continue the setup.", z.w)
				}
			})
		}()
	}, z.w)
}
