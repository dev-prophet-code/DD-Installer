package install

import (
	"bytes"
	"regexp"
	"strings"
)

// redactor removes secrets from everything shown in the GUI or sent to the
// assistant. Besides exact secret values it masks the generated admin
// password that the upstream installer prints, and common token shapes.
type redactor struct {
	secrets  []string
	maskNext bool
}

var tokenLike = regexp.MustCompile(`(?i)(token|password|passwd|secret|authorization)(["']?\s*[:=]\s*["']?)([^\s"']{6,})`)

func newRedactor(secrets ...string) *redactor {
	r := &redactor{}
	for _, s := range secrets {
		if len(s) >= 4 {
			r.secrets = append(r.secrets, s)
		}
	}
	return r
}

// apply masks secrets in free text (no line state).
func (r *redactor) apply(s string) string {
	for _, sec := range r.secrets {
		s = strings.ReplaceAll(s, sec, "***")
	}
	return tokenLike.ReplaceAllString(s, "${1}${2}***")
}

// line masks one output line, remembering the "next line is a password" state.
func (r *redactor) line(s string) string {
	if r.maskNext {
		r.maskNext = false
		if strings.TrimSpace(s) != "" {
			return "  ********"
		}
	}
	if strings.Contains(s, "Use this password to sign in") {
		r.maskNext = true
	}
	return r.apply(s)
}

// lineWriter splits a byte stream into lines.
type lineWriter struct {
	buf []byte
	fn  func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			if len(w.buf) > 64*1024 { // runaway line without newline
				w.fn(string(w.buf))
				w.buf = nil
			}
			break
		}
		w.fn(strings.TrimRight(string(w.buf[:i]), "\r"))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.fn(strings.TrimRight(string(w.buf), "\r"))
		w.buf = nil
	}
}
