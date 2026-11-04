package transport

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
)

// Local runs scripts through a configurable shell command. On Linux this is
// plain `sh -s`; platforms/windows wraps it with `wsl.exe -d <distro> --exec sh -c ...`.
type Local struct {
	Label string
	Cmd   []string // e.g. {"sh","-s"} or {"wsl.exe","-d","Ubuntu","--","sh","-s"}
}

// NewLocal returns a runner for the local Linux shell.
func NewLocal() *Local { return &Local{Label: "local", Cmd: ShellArgs} }

func (l *Local) Describe() string { return l.Label }
func (l *Local) Close() error     { return nil }

func (l *Local) Run(ctx context.Context, script string, out io.Writer) (int, error) {
	if len(l.Cmd) == 0 {
		return -1, errors.New("local runner has no command")
	}
	cmd := exec.CommandContext(ctx, l.Cmd[0], l.Cmd[1:]...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}
