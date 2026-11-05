//go:build windows

package transport

import (
	"fmt"
	"io"
	"os"
)

// openSSHAgentPipe is the named pipe of the Windows OpenSSH agent service.
const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent connects to the Windows OpenSSH agent (service "ssh-agent") or,
// when SSH_AUTH_SOCK names a pipe, to that one. Pageant is not supported.
func dialAgent() (io.ReadWriteCloser, error) {
	path := openSSHAgentPipe
	if p := os.Getenv("SSH_AUTH_SOCK"); len(p) > 9 && p[:9] == `\\.\pipe\` {
		path = p
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the Windows ssh-agent (%v). Start it once as administrator: Get-Service ssh-agent | Set-Service -StartupType Automatic; Start-Service ssh-agent - then add your key with ssh-add", err)
	}
	return f, nil
}
