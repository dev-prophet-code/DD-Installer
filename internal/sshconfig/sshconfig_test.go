package sshconfig

import (
	"strings"
	"testing"
)

const sample = `
# my servers
Host dune
    HostName 203.0.113.10
    User dune
    Port 2222
    IdentityFile "~/.ssh/dune key"
    ProxyJump bastion@198.51.100.5:2200

Host *.internal !secret.internal
    User ops

Host *
    User fallback
    Port 22
    IdentityFile ~/.ssh/id_default
`

func TestLookup(t *testing.T) {
	c := Parse(strings.NewReader(sample))
	e, found := c.Lookup("dune")
	if !found || e.HostName != "203.0.113.10" || e.User != "dune" || e.Port != 2222 || e.IdentityFile != "~/.ssh/dune key" || e.ProxyJump != "bastion@198.51.100.5:2200" {
		t.Fatalf("dune: %+v found=%v", e, found)
	}
	e, found = c.Lookup("db.internal")
	if found || e.User != "ops" || e.Port != 22 || e.HostName != "db.internal" {
		t.Fatalf("wildcard host: %+v found=%v", e, found)
	}
	e, _ = c.Lookup("secret.internal")
	if e.User != "fallback" {
		t.Fatalf("negated pattern must not match: %+v", e)
	}
	e, found = c.Lookup("unknown")
	if found || e.User != "fallback" || e.HostName != "unknown" {
		t.Fatalf("unknown: %+v", e)
	}
}

func TestFirstValueWinsAndNone(t *testing.T) {
	c := Parse(strings.NewReader("Host a\n User first\n User second\n ProxyJump none\nHost a\n User third\n ProxyJump x\n"))
	e, _ := c.Lookup("a")
	if e.User != "first" || e.ProxyJump != "x" {
		// "none" in the first block disables only that block's value; the next block may set one
		t.Fatalf("%+v", e)
	}
}

func TestAliasesAndGarbage(t *testing.T) {
	c := Parse(strings.NewReader("Host a b *\nHost c\n=\n  =  \nMatch all\n User ignored\nHost d\n Port notanumber\n"))
	if got := strings.Join(c.Aliases(), ","); got != "a,b,c,d" {
		t.Fatalf("aliases: %s", got)
	}
	if e, _ := c.Lookup("d"); e.Port != 0 {
		t.Fatal("invalid port must be ignored")
	}
	if e, _ := c.Lookup("a"); e.User != "" {
		t.Fatal("Match blocks must be ignored")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	c, err := Load()
	if err != nil || len(c.Aliases()) != 0 {
		t.Fatal("a missing config must be fine")
	}
}
