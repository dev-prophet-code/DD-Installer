//go:build linux

// Package linux is the Linux implementation: Dune Docker runs natively here,
// so local installs execute the upstream installer directly.
package linux

import (
	"os"
	"os/exec"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
	"github.com/dev-prophet-code/dd-installer/platforms"
)

type linux struct{}

func init() { platform.Register(linux{}) }

func (linux) ID() string          { return "linux" }
func (linux) DisplayName() string { return "Linux" }

func (linux) LocalSupport() platform.LocalSupport {
	if b, err := os.ReadFile("/proc/version"); err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft") {
		return platform.LocalSupport{Supported: true, Reason: "Running inside WSL2: make sure Docker Desktop's WSL integration or a native Docker engine is available."}
	}
	if os.Geteuid() == 0 {
		return platform.LocalSupport{Reason: "Running as root. Start the installer as a regular user with sudo access."}
	}
	return platform.LocalSupport{Supported: true}
}

func (linux) LocalRunner(model.Config) (transport.Runner, error) { return transport.NewLocal(), nil }

func (linux) OpenBrowser(url string) error {
	for _, c := range []string{"xdg-open", "gio", "sensible-browser"} {
		if p, err := exec.LookPath(c); err == nil {
			args := []string{url}
			if c == "gio" {
				args = []string{"open", url}
			}
			return exec.Command(p, args...).Start()
		}
	}
	return exec.ErrNotFound
}

func (linux) DefaultSSHKeys() []string { return platforms.SSHKeyCandidates() }
