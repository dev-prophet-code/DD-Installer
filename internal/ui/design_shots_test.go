package ui

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/sshkey"
)

func TestShots(t *testing.T) {
	dir := os.Getenv("DD_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a := test.NewApp()
	a.Settings().SetTheme(duneTheme{})
	pf := &fakePF{id: "windows", ls: platform.LocalSupport{Reason: "No Ubuntu WSL2 distribution found.", CanProvision: true, ProvisionHint: "x"}}
	w, z := newWizard(a, app.New(pf))
	size := fyne.NewSize(1020, 820)
	w.Resize(size)
	w.Canvas().(test.WindowlessCanvas).Resize(size)
	save := func(name string) {
		w.Canvas().(test.WindowlessCanvas).Resize(fyne.NewSize(1021, 821))
		w.Canvas().(test.WindowlessCanvas).Resize(size)
		img := w.Canvas().Capture()
		f, _ := os.Create(dir + "/" + name + ".png")
		defer f.Close()
		_ = png.Encode(f, img)
	}
	save("p0")
	z.show(1)
	z.token.SetText("x")
	save("p1")
	z.show(2)
	z.addSietch("Atreides", "PVP")
	z.dd.SetSelected("3")
	save("p2")
	z.show(3)
	save("p3")
	z.show(4)
	z.onEvent2()
	save("p4")
	z.showSSHHelp()
	save("help")
	z.showKeyCreated(sshkey.Result{PrivatePath: "/home/me/.ssh/dd-installer_ed25519", PublicKey: "ssh-ed25519 AAAAC3Nza dd-installer", Fingerprint: "SHA256:abc"})
	save("keydone")
}
