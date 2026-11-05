package ui

import (
	"context"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/manage"
)

// showManage opens the management dialog for the server entered on the Target page.
func (z *wizard) showManage() {
	if errs := z.pageErrors(pgTarget); len(errs) > 0 {
		z.show(pgTarget)
		z.setErr("Enter the connection first:\n" + strings.Join(errs, "\n"))
		return
	}
	if z.c.Running() {
		dialog.ShowInformation("Busy", "An installation is running. Wait until it has finished.", z.w)
		return
	}
	acts := manage.Actions()
	var titles []string
	byTitle := map[string]manage.Action{}
	for _, a := range acts {
		label := "[" + string(a.Group) + "] " + a.Title
		titles = append(titles, label)
		byTitle[label] = a
	}
	sel := widget.NewSelect(titles, nil)
	desc := widget.NewLabel("Choose an action.")
	desc.Wrapping = fyne.TextWrapWord
	risk := widget.NewLabel("")
	phrase := widget.NewEntry()
	phraseRow := container.NewVBox()
	logE := widget.NewMultiLineEntry()
	logE.TextStyle = fyne.TextStyle{Monospace: true}
	logE.Wrapping = fyne.TextWrapBreak
	logE.SetMinRowsVisible(10)
	var lines []string
	run := widget.NewButton("Run", nil)
	run.Importance = widget.HighImportance
	run.Disable()

	sel.OnChanged = func(t string) {
		a := byTitle[t]
		desc.SetText(a.Description)
		switch a.Risk {
		case manage.Safe:
			risk.SetText("Risk: harmless")
			risk.Importance = widget.SuccessImportance
		case manage.Disruptive:
			risk.SetText("Risk: restarts services - connected players are disconnected")
			risk.Importance = widget.WarningImportance
		case manage.Destructive:
			risk.SetText("Risk: DELETES DATA - cannot be undone")
			risk.Importance = widget.DangerImportance
		}
		phraseRow.Objects = nil
		if a.Phrase != "" {
			phrase.SetText("")
			phrase.SetPlaceHolder("type " + a.Phrase + " to confirm")
			phraseRow.Add(phrase)
		}
		phraseRow.Refresh()
		run.Enable()
	}
	run.OnTapped = func() {
		a := byTitle[sel.Selected]
		lines = nil
		logE.SetText("")
		run.Disable()
		req := z.request()
		go func() {
			code, err := z.c.RunAction(context.Background(), req, a.ID, phrase.Text, func(l string) {
				fyne.Do(func() {
					lines = append(lines, l)
					if len(lines) > 2000 {
						lines = lines[len(lines)-2000:]
					}
					logE.SetText(strings.Join(lines, "\n"))
					logE.CursorRow = len(lines)
				})
			})
			fyne.Do(func() {
				run.Enable()
				switch {
				case err != nil:
					lines = append(lines, "", "Not run: "+err.Error())
				case code != 0:
					lines = append(lines, "", "Finished with exit code "+itoa(code)+" - see the output above.")
				default:
					lines = append(lines, "", "Done.")
				}
				logE.SetText(strings.Join(lines, "\n"))
			})
		}()
	}
	content := container.NewVBox(
		widget.NewLabel("Server: "+z.request().Cfg().SSH.Host), sel, desc, risk, phraseRow, run, logE)
	sc := container.NewVScroll(content)
	sc.SetMinSize(fyne.NewSize(700, 520))
	dialog.ShowCustom("Manage an existing server", "Close", sc, z.w)
}

// runActionDialog runs one action and shows its output (used by the finish page).
func (z *wizard) runActionDialog(id, title string) {
	logE := widget.NewMultiLineEntry()
	logE.TextStyle = fyne.TextStyle{Monospace: true}
	logE.Wrapping = fyne.TextWrapBreak
	logE.SetMinRowsVisible(16)
	logE.SetText("Running...")
	sc := container.NewVScroll(logE)
	sc.SetMinSize(fyne.NewSize(680, 420))
	dialog.ShowCustom(title, "Close", sc, z.w)
	var lines []string
	req := z.request()
	go func() {
		code, err := z.c.RunAction(context.Background(), req, id, "", func(l string) { lines = append(lines, l) })
		fyne.Do(func() {
			if err != nil {
				lines = append(lines, "", "Not run: "+err.Error())
			} else if code != 0 {
				lines = append(lines, "", "Finished with exit code "+itoa(code))
			}
			logE.SetText(strings.Join(lines, "\n"))
		})
	}()
}
