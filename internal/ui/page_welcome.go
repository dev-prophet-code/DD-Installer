package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	urlDocs   = "https://docs.dunedocker.app/"
	urlGuide  = "https://github.com/dev-prophet-code/DD-Installer/blob/main/docs/SSH-GUIDE.md"
	urlRouter = "https://github.com/dev-prophet-code/DD-Installer/blob/main/docs/ROUTER-AND-FIREWALL.md"
	urlFAQ    = "https://github.com/dev-prophet-code/DD-Installer/blob/main/docs/FAQ.md"
)

func link(text, raw string) *widget.Hyperlink {
	u, _ := url.Parse(raw)
	return widget.NewHyperlink(text, u)
}

func bullets(items ...string) fyne.CanvasObject {
	var objs []fyne.CanvasObject
	for _, it := range items {
		l := widget.NewLabel("◆  " + it)
		l.Wrapping = fyne.TextWrapWord
		objs = append(objs, l)
	}
	return container.NewVBox(objs...)
}

func (z *wizard) pageWelcome() fyne.CanvasObject {
	intro := widget.NewLabel("This assistant installs and sets up your own Dune: Awakening server (Dune Docker) - on this computer or on a remote Linux server - and creates your Sietches and Deep Desert. It checks your system, tunes the settings and solves common problems on its own.")
	intro.Wrapping = fyne.TextWrapWord

	need := widget.NewCard("What you need", "", bullets(
		"Your Funcom self-host service token (from your Funcom self-hosting account)",
		"A Linux server (Ubuntu recommended) with a normal user that may use sudo - or Windows with WSL2 / Linux on this computer",
		"x86-64 CPU with AVX2, at least 20 GB RAM (30-40 GB with more Sietches or Deep Desert) and 200 GB free disk",
		"About 30-90 minutes; the game download is large. You can close the window: the server keeps working",
	))
	happens := widget.NewCard("What will happen", "", bullets(
		"Target: choose where to install (and connect over SSH if needed)",
		"Server: name, region and your token",
		"Layout: how many Sietches, their names and PvE/PvP, Deep Desert",
		"Scan: your system is checked and optimised",
		"Install: everything is set up; problems are explained and fixed where safe",
		"Finish: console address, admin password and a port checklist",
	))
	links := container.NewHBox(link("Official documentation", urlDocs), link("SSH guide for beginners", urlGuide),
		link("Router & firewall guide", urlRouter), link("FAQ", urlFAQ))
	return container.NewVScroll(container.NewVBox(heading("Welcome", 24), intro, need, happens, links))
}
