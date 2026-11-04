package ui

import (
	"fmt"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/sshkey"
)

const sshGuideURL = "https://github.com/dev-prophet-code/DD-Installer/blob/main/docs/SSH-GUIDE.md"

const sshHelpText = `SSH lets this program log in to your Linux server securely. You need three things:

1. A server (a VPS or your own Linux machine) with its address (IP or name).
2. A normal user on it that may use sudo - NOT "root".
3. A key pair: a private key (stays on THIS computer, never share it) and a public key (goes onto the server).

Quick start
 a) Click "Create SSH key" below. The program makes the key pair for you and saves it in your ".ssh" folder.
 b) It then shows ONE command. Run it in a terminal (Windows: PowerShell, macOS: Terminal). It asks for the server password once and installs your public key.
 c) Enter Host, User and press "Scan target". Confirm the server fingerprint when asked.

If you only have a password, choose "Password" as authentication - it works too, keys are just safer.

Typical problems
 - "Connection refused" / "timed out": wrong address or port, or the server firewall blocks port 22.
 - "Permission denied": wrong user, or the public key is not installed for that user.
 - "Host key changed": the server was reinstalled - or someone is intercepting. Ask the owner before continuing.

The full step-by-step guide (with every command for Windows, macOS and Linux) is on GitHub.`

func (z *wizard) showSSHHelp() {
	l := widget.NewLabel(sshHelpText)
	l.Wrapping = fyne.TextWrapWord
	u, _ := url.Parse(sshGuideURL)
	sc := container.NewVScroll(container.NewVBox(l, widget.NewHyperlink("Open the full SSH guide", u)))
	sc.SetMinSize(fyne.NewSize(620, 430))
	dialog.ShowCustom("SSH help - how to connect to your server", "Close", sc, z.w)
}

// showKeyCreator lets users without an SSH key create one with a click.
func (z *wizard) showKeyCreator() {
	name := widget.NewEntry()
	name.SetText(sshkey.DefaultName)
	pass := widget.NewPasswordEntry()
	pass.SetPlaceHolder("optional but recommended")
	pass2 := widget.NewPasswordEntry()
	dir, derr := sshkey.DefaultDir()
	where := widget.NewLabel("The key pair is saved in: " + dir + "\n(the private key is created with owner-only permissions; existing keys are never overwritten)")
	where.Wrapping = fyne.TextWrapWord
	form := widget.NewForm(
		widget.NewFormItem("Key name", name),
		widget.NewFormItem("Passphrase", pass),
		widget.NewFormItem("Repeat passphrase", pass2))
	content := container.NewVBox(where, form)
	d := dialog.NewCustomConfirm("Create SSH key", "Create key", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		if derr != nil {
			dialog.ShowError(derr, z.w)
			return
		}
		if pass.Text != pass2.Text {
			dialog.ShowError(fmt.Errorf("the two passphrases do not match"), z.w)
			return
		}
		res, err := sshkey.Generate(dir, strings.TrimSpace(name.Text), pass.Text, "dd-installer")
		if err != nil {
			dialog.ShowError(err, z.w)
			return
		}
		z.auth.SetSelected(authKey)
		z.key.SetText(res.PrivatePath)
		z.phrase.SetText(pass.Text) // convenience for this session only; never written to disk
		z.showKeyCreated(res)
	}, z.w)
	d.Show()
}

func (z *wizard) showKeyCreated(res sshkey.Result) {
	user, host := strings.TrimSpace(z.user.Text), strings.TrimSpace(z.host.Text)
	if user == "" {
		user = "USER"
	}
	if host == "" {
		host = "SERVER-ADDRESS"
	}
	cmds := sshkey.InstallCommands(res.PublicKey, user, host)

	pub := widget.NewMultiLineEntry()
	pub.SetText(res.PublicKey)
	pub.Wrapping = fyne.TextWrapBreak
	pub.SetMinRowsVisible(3)

	tabs := container.NewAppTabs()
	for _, k := range []string{"Windows (PowerShell)", "macOS / Linux", "On the server itself"} {
		cmd := cmds[k]
		e := widget.NewMultiLineEntry()
		e.SetText(cmd)
		e.Wrapping = fyne.TextWrapBreak
		e.SetMinRowsVisible(4)
		cp := widget.NewButton("Copy command", func() { z.a.Clipboard().SetContent(cmd) })
		tabs.Append(container.NewTabItem(k, container.NewBorder(nil, cp, nil, nil, e)))
	}
	intro := widget.NewLabel("Your key pair was created.\n\nPrivate key (stays on this computer): " + res.PrivatePath +
		"\nFingerprint: " + res.Fingerprint +
		"\n\nNext step: put the PUBLIC key on your server. Run ONE of the commands below in a terminal on this computer (it asks for your server password once):")
	intro.Wrapping = fyne.TextWrapWord
	note := widget.NewLabel("Never send or upload the private key. The path was filled in for you; after the command succeeded, press \"Scan target\".")
	note.Wrapping = fyne.TextWrapWord
	cpPub := widget.NewButton("Copy public key", func() { z.a.Clipboard().SetContent(res.PublicKey) })
	sc := container.NewVScroll(container.NewVBox(intro, tabs, widget.NewLabel("Public key (safe to share, e.g. for a hosting panel):"), pub, cpPub, note))
	sc.SetMinSize(fyne.NewSize(680, 460))
	dialog.ShowCustom("SSH key created", "Done", sc, z.w)
}
