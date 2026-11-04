package ui

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
	"github.com/dev-prophet-code/dd-installer/internal/sshkey"
)

// TestShots renders every page and dialog with realistic example data into
// the directory named by DD_SHOTS (used for docs/images in the README):
//
//	DD_SHOTS=$PWD/docs/images go test ./internal/ui -run TestShots
func TestShots(t *testing.T) {
	dir := os.Getenv("DD_SHOTS")
	if dir == "" {
		t.Skip("set DD_SHOTS to a directory to render screenshots")
	}
	a := test.NewApp()
	a.Settings().SetTheme(duneTheme{})
	pf := &fakePF{id: "windows", name: "Windows", ls: platform.LocalSupport{Reason: "No Ubuntu WSL2 distribution found.", CanProvision: true, ProvisionHint: "x"}}
	w, z := newWizard(a, app.New(pf))
	size := fyne.NewSize(1020, 820)
	w.Resize(size)
	cv := w.Canvas().(test.WindowlessCanvas)
	save := func(name string) {
		cv.Resize(fyne.NewSize(1021, 821)) // force a full layout pass
		cv.Resize(size)
		f, err := os.Create(dir + "/" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, cv.Capture())
	}

	// 1 target
	z.target.SetSelected(optSSH)
	z.host.SetText("203.0.113.10")
	z.user.SetText("dune")
	z.key.SetText(`C:\Users\You\.ssh\dd-installer_ed25519`)
	save("01-target")

	// 2 server
	z.show(1)
	z.title.SetText("Spice Harvesters EU")
	z.region.SetSelected("Europe")
	z.token.SetText("funcom-example-token")
	save("02-server")

	// 3 layout
	z.show(2)
	z.rows[0].name.SetText("Arrakeen Rise")
	z.addSietch("Harko Outpost", "PVP")
	z.addSietch("Sietch Tabr", "PVE")
	z.dd.SetSelected("3")
	z.dd3.SetSelected("PVP")
	save("03-layout")

	// 4 scan
	z.show(3)
	info := probe.Info{OS: "Linux", Arch: "x86_64", Distro: "Ubuntu 24.04 LTS", User: "dune", UID: 1000, CPUModel: "AMD EPYC 7543 32-Core", CPUThreads: 16,
		AVX: true, AVX2: true, MemTotalKB: 64 * 1024 * 1024, DiskFreeKB: 420 * 1024 * 1024, SwapTotalKB: 0, Systemd: true, Docker: false, Tools: map[string]bool{"curl": true, "tar": true}}
	cfg := z.request().Cfg()
	z.showScan(app.ScanResult{Target: "dune@203.0.113.10:22", Host: info, Report: sizing.Analyse(info, cfg)})
	save("04-scan")

	// 5 install in progress with the assistant at work
	z.show(4)
	z.accept.SetChecked(true)
	z.onEvent(install.Event{Type: "step", Step: "bootstrap", Status: "ok", Msg: "Download Dune Docker"})
	z.onEvent(install.Event{Type: "step", Step: "install", Status: "ok", Msg: "Install Docker & start the Web UI (retry 1)"})
	z.onEvent(install.Event{Type: "step", Step: "tuning", Status: "ok", Msg: "Apply performance tuning"})
	z.onEvent(install.Event{Type: "step", Step: "console-setup", Status: "running", Msg: "Configure server & deploy game files"})
	d := assistant.Diagnosis{RuleID: "port-in-use", Title: "Port already in use", Risk: assistant.RiskSafe,
		Explain: "Another service uses the chosen web port. The installer picks the next free port and retries.",
		Actions: []assistant.Action{{Description: "Use the next free Web UI port and retry."}}, DocURL: "https://docs.dunedocker.app/"}
	z.onEvent(install.Event{Type: "diagnosis", Diagnosis: &d})
	for _, l := range []string{"==> Choosing the Web UI port.", "Port 8088 is already in use. Using available port 8089.", "Server identity saved.", "Funcom token saved.", "Deploying (downloads the game files - this can take a long time)..."} {
		z.onEvent(install.Event{Type: "log", Msg: l})
	}
	z.mote.setStorm(true)
	save("05-install")
	z.mote.setStorm(false)

	// 6 SSH key helper
	z.show(0)
	z.showKeyCreated(sshkey.Result{PrivatePath: `C:\Users\You\.ssh\dd-installer_ed25519`, PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample dd-installer", Fingerprint: "SHA256:Qk3x0Example"})
	save("06-ssh-key")

	// 7 WSL offer on a fresh Windows
	_ = model.Default
	z2 := newTest(t, &fakePF{id: "windows", name: "Windows", ls: platform.LocalSupport{Reason: "No Ubuntu WSL2 distribution found.", CanProvision: true, ProvisionHint: "x"}})
	z2.target.SetSelected(optLocal)
	z2.w.Resize(size)
	c2 := z2.w.Canvas().(test.WindowlessCanvas)
	c2.Resize(fyne.NewSize(1021, 821))
	c2.Resize(size)
	f, _ := os.Create(dir + "/07-wsl.png")
	defer f.Close()
	_ = png.Encode(f, c2.Capture())
}
