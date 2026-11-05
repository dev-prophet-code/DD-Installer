package model

import (
	"strings"
	"testing"
)

func FuzzCheckToken(f *testing.F) {
	f.Add("abcdefghijklmnopqrstuvwxyz0123456789")
	f.Add("  \t\n")
	f.Add("tok\u00e9n$(id)")
	f.Fuzz(func(t *testing.T, raw string) {
		tok, problem, _ := CheckToken(raw)
		if problem != "" {
			return
		}
		if tok != strings.TrimSpace(raw) || tok == "" {
			t.Fatalf("accepted token must be the trimmed input: %q -> %q", raw, tok)
		}
		for _, r := range tok {
			if r < 0x21 || r > 0x7e || strings.ContainsRune("\"'`$\\", r) {
				t.Fatalf("accepted token contains %q", r)
			}
		}
	})
}

// A config that passes Validate must never contain shell metacharacters in the
// fields that end up inside generated scripts.
func FuzzValidateIsSafe(f *testing.F) {
	f.Add("~/dune", "My Server", "Sietch 1", "203.0.113.0/24", "user@jump:22")
	f.Add("/a;b", "t\"x", "$(id)", "x", "a,b")
	f.Fuzz(func(t *testing.T, dir, title, sietch, allow, jump string) {
		c := Default()
		c.InstallDir, c.ServerTitle = dir, title
		c.Sietches = []Sietch{{Name: sietch, Mode: PvE}}
		c.ConsoleAllowFrom = allow
		c.Target = TargetSSH
		c.SSH = SSHConfig{Host: "h", Port: 22, User: "u", Auth: AuthAgent, Jump: jump}
		if len(c.Validate()) != 0 {
			return
		}
		for _, v := range []string{c.InstallDir, c.ServerTitle, c.ConsoleAllowFrom, c.SSH.Jump, c.Sietches[0].Name} {
			if strings.ContainsAny(v, "`$\\;|&<>\n\r") {
				t.Fatalf("valid config with unsafe value %q", v)
			}
		}
		if strings.ContainsAny(c.InstallDir, `'"`) {
			t.Fatalf("install dir with quotes accepted: %q", c.InstallDir)
		}
	})
}

func FuzzFieldValidators(f *testing.F) {
	f.Add("22")
	f.Add("../../x")
	f.Fuzz(func(t *testing.T, s string) {
		_ = ValidSietchName(s)
		_ = ValidTitle(s)
		_ = ValidPort(s)
		_ = ValidHost(s)
		_ = ValidUser(s)
		_ = ValidAllowFrom(s)
	})
}
