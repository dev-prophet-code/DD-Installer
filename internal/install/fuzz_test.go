package install

import (
	"strings"
	"testing"
	"time"
)

// Secrets must never survive redaction, whatever surrounds them.
func FuzzRedactorHidesSecrets(f *testing.F) {
	f.Add("TOPSECRET1234", "prefix TOPSECRET1234 suffix")
	f.Add("a$b'c\"d", "x a$b'c\"d y a$b'c\"d")
	f.Fuzz(func(t *testing.T, secret, text string) {
		if len(secret) < 4 {
			return
		}
		r := newRedactor(secret)
		if got := r.apply(text); strings.Contains(got, secret) {
			t.Fatalf("secret %q survived in %q", secret, got)
		}
		if got := r.line(text); strings.Contains(got, secret) {
			t.Fatalf("secret %q survived in line %q", secret, got)
		}
	})
}

// The line splitter must not lose or invent text.
func FuzzLineWriter(f *testing.F) {
	f.Add("a\nb\r\nc", 3)
	f.Add(strings.Repeat("x", 70000), 5000)
	f.Fuzz(func(t *testing.T, in string, chunk int) {
		if chunk < 1 {
			chunk = 1
		}
		var lines []string
		w := &lineWriter{fn: func(s string) { lines = append(lines, s) }}
		for i := 0; i < len(in); i += chunk {
			end := i + chunk
			if end > len(in) {
				end = len(in)
			}
			w.Write([]byte(in[i:end]))
		}
		w.Flush()
		joined := strings.Join(lines, "")
		want := strings.NewReplacer("\n", "", "\r", "").Replace(in)
		if strings.ReplaceAll(joined, "\r", "") != strings.ReplaceAll(want, "\r", "") {
			t.Fatalf("text changed by the line writer")
		}
	})
}

func FuzzParsePoll(f *testing.F) {
	f.Add("DD_STATE running\nDD_BODY\nhello")
	f.Add("DD_BODY\n")
	f.Add("DD_STATE exit:3\nDD_BODY\n")
	f.Fuzz(func(t *testing.T, in string) {
		state, body, ok := parsePoll(in)
		if ok && state == "" {
			t.Fatal("ok implies a state")
		}
		if !ok && body != "" {
			t.Fatal("no body without a valid poll")
		}
	})
}

// Environment values are written into .env: only the safe alphabet may pass.
func FuzzEnvValueWhitelist(f *testing.F) {
	f.Add("+S 2:2 +sbwt none")
	f.Add("$(id)")
	f.Fuzz(func(t *testing.T, v string) {
		if safeEnvVal(v) && strings.ContainsAny(v, "`$\"'\\;|&<>()\n\r") {
			t.Fatalf("unsafe value accepted: %q", v)
		}
	})
}

// Output that arrives byte by byte without newlines (progress redraws) must not be quadratic.
func TestLineWriterByteByByteIsLinear(t *testing.T) {
	var n int
	w := &lineWriter{fn: func(string) { n++ }}
	start := time.Now()
	data := strings.Repeat("Update state (0x61) downloading, progress: 45.12\r", 4000)
	for i := 0; i < len(data); i++ {
		w.Write([]byte{data[i]})
	}
	w.Flush()
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("byte-by-byte writing took %v", d)
	}
	if n != 4000 {
		t.Fatalf("each carriage-return redraw must become one line, got %d", n)
	}
}
