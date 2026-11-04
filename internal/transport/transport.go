// Package transport runs shell commands on the install target. Three
// implementations exist: Local (this machine, Linux), WSL (Windows, provided
// by platforms/windows) and SSH (any OS, remote Linux server).
package transport

import (
	"context"
	"io"
)

// ShellArgs is how every runner starts the target shell. The script arrives
// on stdin; `eval "$(cat)"` reads ALL of it first so that child processes
// (apt, sudo, docker, ...) cannot swallow the unread rest of the script from
// a shared stdin, and they see EOF instead.
var ShellArgs = []string{"sh", "-c", `eval "$(cat)"`}

// ShellCommand is ShellArgs as one command line for SSH.
const ShellCommand = `sh -c 'eval "$(cat)"'`

// Runner executes POSIX shell snippets on the target.
type Runner interface {
	// Run executes script via `sh -s` on the target, feeding it on stdin.
	// Combined stdout/stderr is streamed to out. A non-zero exit status is
	// returned as exit with a nil error.
	Run(ctx context.Context, script string, out io.Writer) (exit int, err error)
	// Describe is a human-readable target label ("user@host:22", "local").
	Describe() string
	Close() error
}

// Capture runs script and returns its output as a string.
func Capture(ctx context.Context, r Runner, script string) (string, int, error) {
	var b limitedBuffer
	code, err := r.Run(ctx, script, &b)
	return b.String(), code, err
}

type limitedBuffer struct{ buf []byte }

const maxCapture = 1 << 20

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := maxCapture - len(b.buf); room > 0 {
		if len(p) > room {
			b.buf = append(b.buf, p[:room]...)
		} else {
			b.buf = append(b.buf, p...)
		}
	}
	return len(p), nil
}
func (b *limitedBuffer) String() string { return string(b.buf) }
