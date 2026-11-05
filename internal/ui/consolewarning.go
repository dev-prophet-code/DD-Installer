package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// The Dune Docker web console speaks plain HTTP. Before an installation starts,
// the user has to read and acknowledge what that means for the admin password
// and for who can reach the console.

// consoleWarningText returns the paragraphs of the notice for this plan.
func consoleWarningText(cfg model.Config) (intro, exposure string, advice []string) {
	port := cfg.WebPort
	intro = "The Dune Docker web console is served over plain HTTP, not HTTPS. The admin password and everything you do in the console travel unencrypted. " +
		"Anyone who can watch the network path between you and the server can read them."

	switch {
	case cfg.Target != model.TargetSSH:
		exposure = fmt.Sprintf("The console runs on this computer (http://localhost:%d). Do not forward its port to the internet.", port)
	case strings.TrimSpace(cfg.ConsoleAllowFrom) != "":
		exposure = fmt.Sprintf("The console port %d will only accept connections from %s. That is the safer choice, but the traffic is still unencrypted.", port, strings.TrimSpace(cfg.ConsoleAllowFrom))
	case cfg.ConfigureFirewall:
		exposure = fmt.Sprintf("Nothing limits who can open the console: DD-Installer will open port %d in the server firewall for EVERYONE. Anybody who finds the server can try to sign in.", port)
	default:
		exposure = fmt.Sprintf("Nothing limits who can open the console on port %d. Docker publishes the port itself, so a host firewall such as ufw may not block it. Check who can reach it.", port)
	}

	advice = []string{"Restrict the console to your own address (step \"Scan\": limit the web console to your IP or network)."}
	if cfg.Target == model.TargetSSH {
		advice = append(advice,
			fmt.Sprintf("Or do not open it to the internet at all and use an SSH tunnel instead:\n    ssh -L %d:127.0.0.1:%d %s@%s\n  then open http://localhost:%d", port, port, cfg.SSH.User, cfg.SSH.Host, port),
			"A VPN to the server network works as well.")
	}
	advice = append(advice, "Use a password for the console that you do not use anywhere else.")
	return intro, exposure, advice
}

// consoleNotice is the content of the notice and the widgets that matter.
type consoleNotice struct {
	content      fyne.CanvasObject
	ack          *widget.Check
	cont, cancel *widget.Button
}

// newConsoleNotice builds the notice. The Continue button stays disabled until
// the box is ticked; onContinue runs only from that button.
func newConsoleNotice(cfg model.Config, onContinue, onCancel func()) *consoleNotice {
	intro, exposure, advice := consoleWarningText(cfg)
	wrap := func(s string) *widget.Label {
		l := widget.NewLabel(s)
		l.Wrapping = fyne.TextWrapWord
		return l
	}
	exp := wrap(exposure)
	exp.Importance = widget.WarningImportance
	adv := make([]fyne.CanvasObject, 0, len(advice))
	for _, a := range advice {
		adv = append(adv, wrap("•  "+a))
	}
	n := &consoleNotice{}
	n.cont = widget.NewButton("I understand - continue", onContinue)
	n.cont.Importance = widget.HighImportance
	n.cont.Disable()
	n.cancel = widget.NewButton("Cancel", onCancel)
	n.ack = widget.NewCheck("I understand that the web console is not encrypted.", func(on bool) {
		if on {
			n.cont.Enable()
		} else {
			n.cont.Disable()
		}
	})
	body := container.NewVBox(
		heading("Web console: not encrypted", 22),
		wrap(intro), exp,
		caption(spaced("How to stay safe"), colSpice, 13),
		container.NewVBox(adv...))
	sc := container.NewVScroll(body)
	sc.SetMinSize(fyne.NewSize(560, 340))
	// the acknowledgement stays visible below the scrolling text
	n.content = container.NewBorder(nil,
		container.NewVBox(widget.NewSeparator(), n.ack, container.NewHBox(n.cancel, n.cont)), nil, nil, sc)
	return n
}

// confirmConsoleExposure shows the notice and calls proceed only when the user
// has ticked the box and pressed Continue.
func (z *wizard) confirmConsoleExposure(cfg model.Config, proceed func()) {
	var d dialog.Dialog
	n := newConsoleNotice(cfg, func() { d.Hide(); proceed() }, func() { d.Hide() })
	d = dialog.NewCustomWithoutButtons("Before you install", n.content, z.w)
	d.Show()
}
