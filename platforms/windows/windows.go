//go:build windows

// Package windows is the Windows implementation. Local installs run inside an
// Ubuntu WSL2 distribution (wsl.exe); Docker Desktop's internal distribution
// is never used.
package windows

import (
	"errors"
	"os/exec"
	"strings"
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
func distros() []string {
	out, err := exec.Command("wsl.exe", "-l", "-q").Output()
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
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return platform.LocalSupport{Reason: "WSL2 is not installed. Run `wsl --install -d Ubuntu` in an administrator PowerShell, or install on a remote server over SSH."}
	}
	d := distros()
	if len(d) == 0 {
		return platform.LocalSupport{Reason: "No Ubuntu WSL2 distribution found. Run `wsl --install -d Ubuntu`, create your Linux user and restart the installer."}
	}
	return platform.LocalSupport{Supported: true, Distros: d,
		Reason: "Installs inside WSL2. Memory is limited by .wslconfig (the scan suggests values)."}
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

func (win) OpenBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func (win) DefaultSSHKeys() []string { return platforms.SSHKeyCandidates() }
