//go:build !windows

package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

// dialAgent connects to the running ssh-agent (SSH_AUTH_SOCK).
func dialAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK is not set; start an ssh-agent and add your key with ssh-add")
	}
	c, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("ssh-agent: %w", err)
	}
	return c, nil
}
