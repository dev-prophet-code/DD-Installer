//go:build darwin

// Package macos is the macOS implementation. Dune Docker needs x86-64 Linux
// containers with AVX2, which macOS cannot provide reliably (Apple Silicon
// emulation does not expose AVX2), so macOS supports remote (SSH) installs
// only.
package macos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
	"github.com/dev-prophet-code/dd-installer/platforms"
)

type mac struct{}

func init() { platform.Register(mac{}) }

func (mac) ID() string          { return "macos" }
func (mac) DisplayName() string { return "macOS" }

func (mac) LocalSupport() platform.LocalSupport {
	return platform.LocalSupport{
		Supported: false,
		Reason:    "The Dune server images are x86-64 Linux and require AVX2, which macOS (Docker Desktop / Apple Silicon emulation) does not expose. Install on a remote Linux server over SSH instead.",
	}
}

func (mac) LocalRunner(model.Config) (transport.Runner, error) {
	return nil, errors.New("local installation is not supported on macOS; use the SSH target")
}

func (mac) ProvisionLocal(context.Context, func(string)) (bool, error) {
	return false, errors.New("local installation is not supported on macOS")
}

// PreventSleep runs the system tool caffeinate for as long as this process lives.
func (mac) PreventSleep() func() {
	cmd := exec.Command("caffeinate", "-dims", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return func() {}
	}
	return func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
}

func (mac) DefaultSSHKeys() []string { return platforms.SSHKeyCandidates() }
