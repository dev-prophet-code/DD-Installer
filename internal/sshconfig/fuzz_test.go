package sshconfig

import (
	"strings"
	"testing"
)

func FuzzParseAndLookup(f *testing.F) {
	f.Add("Host a\n HostName b\n Port 22\n", "a")
	f.Add("Host *\n User x\nMatch all\n =\n", "zzz")
	f.Add("Host !a *.b ?c\n\tProxyJump none\n", "x.b")
	f.Fuzz(func(t *testing.T, cfg, alias string) {
		c := Parse(strings.NewReader(cfg))
		e, _ := c.Lookup(alias)
		if e.Port < 0 || e.Port > 65535 {
			t.Fatalf("port out of range: %d", e.Port)
		}
		if alias != "" && e.HostName == "" {
			t.Fatal("HostName must default to the alias")
		}
		_ = c.Aliases()
	})
}
