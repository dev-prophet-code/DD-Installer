//go:build windows

// Package windows is the Windows implementation. Local installs run inside an
// Ubuntu WSL2 distribution (wsl.exe); Docker Desktop's internal distribution
// is never used.
package windows

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
	"github.com/dev-prophet-code/dd-installer/platforms"
)

type win struct{}

func init() { platform.Register(win{}) }

func (win) ID() string          { return "windows" }
func (win) DisplayName() string { return "Windows" }

// distros lists installed WSL2 distributions suitable for Dune Docker.
func distros() []string { return distroCache.get(listDistros) }

// distroCache avoids starting wsl.exe again on every click in the wizard.
var distroCache = newListCache(8 * time.Second)

func listDistros() []string {
	cmd := exec.Command("wsl.exe", "-l", "-q")
	transport.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var res []string
	for _, l := range strings.Split(decodeUTF16(out), "\n") {
		l = strings.TrimSpace(strings.Trim(l, "\x00\r"))
		if l == "" || strings.HasPrefix(strings.ToLower(l), "docker-desktop") {
			continue // Docker Desktop's internal distros are unusable
		}
		res = append(res, l)
	}
	return res
}

// wsl.exe prints UTF-16LE.
func decodeUTF16(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u))
}

func (win) LocalSupport() platform.LocalSupport {
	missing := platform.LocalSupport{CanProvision: true, ProvisionHint: provisionHint}
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		missing.Reason = "WSL2 is not installed."
		return missing
	}
	d := distros()
	if len(d) == 0 {
		missing.Reason = "No Ubuntu WSL2 distribution found."
		return missing
	}
	return platform.LocalSupport{Supported: true, Distros: d,
		Reason: "Installs inside WSL2. Memory is limited by .wslconfig (the scan suggests values)."}
}

// ProvisionLocal installs WSL2 + Ubuntu (UAC prompt), prepares a user and
// systemd, then stops the distribution so wsl.conf takes effect.
func (w win) ProvisionLocal(ctx context.Context, log func(string)) (bool, error) {
	if d := pickUbuntu(distros()); d != "" {
		log("Ubuntu (" + d + ") is already installed.")
	} else {
		log("Installing WSL2 and Ubuntu. Please confirm the Windows administrator prompt (UAC)...")
		c := installCommand()
		cmd := exec.CommandContext(ctx, c[0], c[1:]...)
		transport.HideWindow(cmd) // the UAC prompt is shown by Windows itself
		out, err := cmd.CombinedOutput()
		distroCache.invalidate()
		if len(out) > 0 {
			log(strings.TrimSpace(string(out)))
		}
		exit := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else if err != nil {
			return false, err
		}
		if exit != 0 && exit != exitRebootRequired {
			return false, fmt.Errorf("WSL installation failed or was cancelled (exit code %d). Run `wsl --install -d Ubuntu` in an administrator PowerShell and try again", exit)
		}
		if rebootNeeded(exit, distros()) {
			log("Windows must be restarted to finish installing WSL2. Restart, then start DD-Installer again; it will continue from here.")
			return true, nil
		}
	}
	name := pickUbuntu(distros())
	if name == "" {
		return true, nil
	}
	log("Preparing " + name + " (user, sudo, systemd)...")
	r := &transport.Local{Label: "WSL2 root", Cmd: append([]string{"wsl.exe", "-d", name, "--user", "root", "--exec"}, transport.ShellArgs...)}
	var b strings.Builder
	code, err := r.Run(ctx, provisionScript(), &b)
	log(strings.TrimSpace(b.String()))
	if err != nil || code != 0 {
		return false, fmt.Errorf("could not prepare Ubuntu (exit %d): %v", code, err)
	}
	term := exec.CommandContext(ctx, "wsl.exe", "--terminate", name)
	transport.HideWindow(term)
	_ = term.Run() // apply wsl.conf
	distroCache.invalidate()
	log("Ubuntu is ready.")
	return false, nil
}

func (win) LocalRunner(cfg model.Config) (transport.Runner, error) {
	d := cfg.WSLDistro
	if d == "" {
		ds := distros()
		if len(ds) == 0 {
			return nil, errors.New("no usable WSL2 distribution found")
		}
		d = ds[0]
	}
	if strings.HasPrefix(strings.ToLower(d), "docker-desktop") {
		return nil, errors.New("docker-desktop is Docker Desktop's internal distribution; choose an Ubuntu distribution")
	}
	return &transport.Local{Label: "WSL2 (" + d + ")", Cmd: append([]string{"wsl.exe", "-d", d, "--exec"}, transport.ShellArgs...)}, nil
}

// PreventSleep keeps Windows awake (SetThreadExecutionState) until released.
// The request belongs to the thread that made it, so a dedicated goroutine
// owns an OS thread for the whole time.
func (win) PreventSleep() func() {
	const (
		esContinuous     = 0x80000000
		esSystemRequired = 0x00000001
	)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		proc.Call(uintptr(esContinuous | esSystemRequired))
		<-stop
		proc.Call(uintptr(esContinuous))
	}()
	return func() { close(stop); <-done }
}

func (win) DefaultSSHKeys() []string { return platforms.SSHKeyCandidates() }
