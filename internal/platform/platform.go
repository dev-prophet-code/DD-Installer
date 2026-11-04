// Package platform abstracts everything that differs between Linux, macOS and
// Windows. Implementations live in platforms/<os>/ and register themselves
// from build-tagged files in cmd/dd-installer.
package platform

import (
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// LocalSupport says whether Dune Docker can be installed on this computer.
type LocalSupport struct {
	Supported bool     `json:"supported"`
	Reason    string   `json:"reason,omitempty"`  // why not / caveats
	Distros   []string `json:"distros,omitempty"` // Windows: usable WSL2 distributions
}

// Platform is implemented once per operating system.
type Platform interface {
	// ID is "linux", "macos" or "windows".
	ID() string
	DisplayName() string
	LocalSupport() LocalSupport
	// LocalRunner returns a runner for the local install target.
	LocalRunner(cfg model.Config) (transport.Runner, error)
	// OpenBrowser opens url in the user's default browser.
	OpenBrowser(url string) error
	// DefaultSSHKeys lists likely private key paths for the key picker.
	DefaultSSHKeys() []string
}

var current Platform

// Register is called by the platform package of the running OS.
func Register(p Platform) { current = p }

// Current returns the registered platform (nil only in tests).
func Current() Platform { return current }
