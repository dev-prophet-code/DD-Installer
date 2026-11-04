//go:build windows

package transport

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: the child gets no console window.
const createNoWindow = 0x08000000

// HideWindow stops a child process (wsl.exe, powershell.exe, ...) from
// flashing a console window when it is started by this GUI program.
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
