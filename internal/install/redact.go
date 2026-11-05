package install

import (
	"bytes"
	"regexp"
	"strings"
)

// redactor removes secrets from everything shown in the UI or sent to the
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

// lineWriter splits a byte stream into lines. It splits at "\n" and also at a
// lone "\r", because progress displays (SteamCMD, docker pull) redraw one line
// with carriage returns; each redraw becomes its own line.
type lineWriter struct {
	buf  []byte
	scan int // bytes of buf already searched for a newline
	fn   func(string)
}

const maxLine = 64 * 1024

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf[w.scan:], '\n')
		if i < 0 {
			w.scan = len(w.buf)       // only new bytes are searched next time (no quadratic rescans)
			if len(w.buf) > maxLine { // runaway line without a newline
				// cut at the last complete redraw if there is one, so no piece is split in two
				cut := bytes.LastIndexByte(w.buf, '\r') + 1
				if cut == 0 {
					cut = len(w.buf)
				}
				w.emit(w.buf[:cut])
				w.buf, w.scan = append([]byte(nil), w.buf[cut:]...), 0
			}
			break
		}
		i += w.scan
		w.emit(w.buf[:i])
		w.buf, w.scan = w.buf[i+1:], 0
	}
	return len(p), nil
}

// emit sends one physical line, split at carriage returns.
func (w *lineWriter) emit(b []byte) {
	if len(b) == 0 {
		w.fn("") // an empty line stays an empty line
		return
	}
	for _, seg := range bytes.Split(b, []byte{'\r'}) {
		if len(seg) > 0 {
			w.fn(string(seg))
		}
	}
}

func (w *lineWriter) Flush() {
	if len(w.buf) > 0 {
		w.emit(w.buf)
		w.buf, w.scan = nil, 0
	}
}
