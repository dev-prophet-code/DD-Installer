// Package platform abstracts everything that differs between Linux, macOS and
// Windows. Implementations live in platforms/<os>/ and register themselves
// from build-tagged files in cmd/dd-installer.
package platform

import (
	"context"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// LocalSupport says whether Dune Docker can be installed on this computer.
type LocalSupport struct {
	Supported bool     `json:"supported"`
	Reason    string   `json:"reason,omitempty"`  // why not / caveats
	Distros   []string `json:"distros,omitempty"` // Windows: usable WSL2 distributions
	// CanProvision means the installer can set up the missing prerequisite
	// itself (Windows: install WSL2 + Ubuntu); ProvisionHint describes it.
	CanProvision  bool   `json:"canProvision,omitempty"`
	ProvisionHint string `json:"provisionHint,omitempty"`
}

// Platform is implemented once per operating system.
type Platform interface {
	// ID is "linux", "macos" or "windows".
	ID() string
	DisplayName() string
	LocalSupport() LocalSupport
	// LocalRunner returns a runner for the local install target.
	LocalRunner(cfg model.Config) (transport.Runner, error)
	// ProvisionLocal installs the missing prerequisites for a local install
	// (Windows: WSL2 + Ubuntu with a prepared user). log receives progress
	// lines. needsReboot is true when Windows must restart before continuing.
	ProvisionLocal(ctx context.Context, log func(string)) (needsReboot bool, err error)
	// DefaultSSHKeys lists likely private key paths for the key picker.
	DefaultSSHKeys() []string
}

var current Platform

// Register is called by the platform package of the running OS.
func Register(p Platform) { current = p }

// Current returns the registered platform (nil only in tests).
func Current() Platform { return current }
