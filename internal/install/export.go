package install

import (
	"context"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// DirExpr returns the install folder as a double-quote-safe shell expression.
func DirExpr(cfg model.Config) string { return dirExpr(cfg) }

// WithPrelude adds the shared shell helpers (dgrp: run with Docker access).
func WithPrelude(script string) string { return withPrelude(script) }

// Shq single-quotes s for POSIX sh.
func Shq(s string) string { return shq(s) }

// StreamScript runs script on r and passes redacted output lines to emit. It
// is used by the management actions, which are not part of an installation.
func StreamScript(ctx context.Context, r transport.Runner, script string, emit func(line string), secrets ...string) (int, error) {
	red := newRedactor(secrets...)
	w := &lineWriter{fn: func(line string) {
		line = red.line(line)
		if strings.HasPrefix(line, "DD_") {
			return
		}
		emit(line)
	}}
	code, err := r.Run(ctx, script, w)
	w.Flush()
	return code, err
}
