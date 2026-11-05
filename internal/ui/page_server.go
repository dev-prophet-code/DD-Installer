package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
)

type serverPage struct {
	title, webPort, token *widget.Entry
	region                *widget.Select
	latest                *widget.Check
	tokenInfo             *widget.Label
}

func (z *wizard) pageServer() fyne.CanvasObject {
	s := &z.srv
	s.title = widget.NewEntry()
	s.title.SetText("My Dune Server")
	s.title.Validator = model.ValidTitle
	s.region = widget.NewSelect(model.Regions, nil)
	s.region.SetSelected("Europe")
	s.webPort = widget.NewEntry()
	s.webPort.SetText("8088")
	s.webPort.Validator = model.ValidPort
	s.tokenInfo = widget.NewLabel("")
	s.tokenInfo.Wrapping = fyne.TextWrapWord
	s.token = widget.NewPasswordEntry()
	s.token.SetPlaceHolder("Funcom self-host service token")
	s.token.OnChanged = func(string) { z.checkToken() }
	paste := widget.NewButtonWithIcon("Paste", theme.ContentPasteIcon(), func() {
		s.token.SetText(z.a.Clipboard().Content())
	})
	note := widget.NewLabel("The token is kept in memory only and is sent through the encrypted channel to your server.")
	note.Wrapping = fyne.TextWrapWord
	s.latest = widget.NewCheck("Install the newest upstream version instead of the tested one ("+install.UpstreamTag+")", nil)
	latestNote := widget.NewLabel("DD-Installer is tested with Dune Docker " + install.UpstreamTag + ". Newer releases may change commands the installer relies on.")
	latestNote.Wrapping = fyne.TextWrapWord
	return container.NewVScroll(container.NewVBox(
		heading("Your server", 24),
		widget.NewForm(
			widget.NewFormItem("Server title", s.title),
			widget.NewFormItem("Region", s.region),
			widget.NewFormItem("Web console port", s.webPort),
			widget.NewFormItem("Funcom token", container.NewBorder(nil, nil, nil, paste, s.token))),
		s.tokenInfo, note, widget.NewSeparator(), s.latest, latestNote))
}

// checkToken gives immediate feedback while the token is typed or pasted.
func (z *wizard) checkToken() {
	s := &z.srv
	if s.token.Text == "" {
		s.tokenInfo.SetText("")
		return
	}
	tok, problem, warning := model.CheckToken(s.token.Text)
	switch {
	case problem != "":
		s.tokenInfo.Importance = widget.DangerImportance
		s.tokenInfo.SetText(problem)
	case warning != "":
		s.tokenInfo.Importance = widget.WarningImportance
		s.tokenInfo.SetText(warning)
	default:
		s.tokenInfo.Importance = widget.SuccessImportance
		s.tokenInfo.SetText("Token looks fine (" + itoa(len(tok)) + " characters).")
	}
}
