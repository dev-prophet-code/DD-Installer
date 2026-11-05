package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
)

func itoa(n int) string { return strconv.Itoa(n) }

type sietchRow struct {
	box  *fyne.Container
	name *widget.Entry
	mode *widget.Select
}

type layoutPage struct {
	rows     []*sietchRow
	rowsBox  *fyne.Container
	combat   *widget.Check
	dd       *widget.Select
	dd3      *widget.Select
	ddRow    *fyne.Container
	ramLabel *widget.Label
}

func (z *wizard) pageLayout() fyne.CanvasObject {
	l := &z.lay
	l.rowsBox = container.NewVBox()
	add := widget.NewButton("+ Add Sietch", func() { z.addSietch("", "PVE") })
	l.combat = widget.NewCheck("Also apply PvP/PvE per Sietch (experimental - verify in game)", nil)
	l.dd = widget.NewSelect([]string{"Upstream default", "1", "2 (one PvE + one PvP)", "3"}, func(string) { z.updateDD() })
	l.dd3 = widget.NewSelect([]string{"PVE", "PVP"}, func(string) {})
	l.dd3.SetSelected("PVE")
	l.ddRow = container.NewVBox(widget.NewLabel("Third instance ruleset"), l.dd3)
	l.ddRow.Hide()
	note := widget.NewLabel("Upstream keeps the first instance's existing ruleset, gives the second the opposite one and lets you choose only the third.")
	note.Wrapping = fyne.TextWrapWord
	l.ramLabel = widget.NewLabel("")
	l.ramLabel.Wrapping = fyne.TextWrapWord
	l.ramLabel.Importance = widget.WarningImportance
	l.dd.SetSelected("Upstream default")
	z.addSietch("Sietch 1", "PVE")
	return container.NewVScroll(container.NewVBox(
		heading("Sietches & Deep Desert", 24),
		caption(spaced("Hagga Basin sietches - name and ruleset"), colSpice, 13),
		l.rowsBox, add, l.combat,
		widget.NewSeparator(),
		caption(spaced("Deep Desert"), colSpice, 13),
		widget.NewForm(widget.NewFormItem("Instances", l.dd)), l.ddRow, note, l.ramLabel))
}

func (z *wizard) addSietch(name, mode string) {
	l := &z.lay
	if name == "" {
		name = fmt.Sprintf("Sietch %d", len(l.rows)+1)
	}
	r := &sietchRow{name: widget.NewEntry(), mode: widget.NewSelect([]string{"PVE", "PVP"}, func(string) {})}
	r.name.Validator = model.ValidSietchName
	r.name.SetText(name)
	r.name.OnChanged = func(string) { z.estimate() }
	r.mode.SetSelected(mode)
	del := widget.NewButton("Remove", nil)
	r.box = container.NewBorder(nil, nil, nil, container.NewHBox(r.mode, del), r.name)
	del.OnTapped = func() {
		if len(l.rows) <= 1 {
			return
		}
		for i, x := range l.rows {
			if x == r {
				l.rows = append(l.rows[:i], l.rows[i+1:]...)
				break
			}
		}
		l.rowsBox.Remove(r.box)
		z.estimate()
	}
	l.rows = append(l.rows, r)
	l.rowsBox.Add(r.box)
	z.estimate()
}

func (z *wizard) updateDD() {
	l := &z.lay
	if l.ddRow == nil {
		return
	}
	if strings.HasPrefix(l.dd.Selected, "3") {
		l.ddRow.Show()
	} else {
		l.ddRow.Hide()
	}
	z.estimate()
}

func (l *layoutPage) ddInstances() int {
	s := l.dd.Selected
	if s == "" || strings.HasPrefix(s, "Upstream") {
		return 0
	}
	n, _ := strconv.Atoi(s[:1])
	return n
}

func (z *wizard) estimate() {
	if z.lay.ramLabel == nil || z.lay.dd == nil {
		return
	}
	cfg := z.request().Cfg()
	z.lay.ramLabel.SetText(fmt.Sprintf("Estimated RAM needed for this layout: about %d GB (heuristic - the scan checks your server).", sizing.RequiredRAM(cfg)))
}
