// Package platforms holds helpers shared by the OS implementations.
package platforms

import (
	"os"
	"path/filepath"
)

// SSHKeyCandidates returns existing default private key files in ~/.ssh.
func SSHKeyCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var out []string
	for _, n := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		p := filepath.Join(home, ".ssh", n)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}
