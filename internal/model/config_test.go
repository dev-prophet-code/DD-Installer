package model

import (
	"strings"
	"testing"
)

func TestDefaultValid(t *testing.T) {
	c := Default()
	if errs := c.Validate(); len(errs) != 0 {
		t.Fatalf("default config invalid: %v", errs)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(*Config){
		"root ssh":      func(c *Config) { c.Target = TargetSSH; c.SSH.Host = "h"; c.SSH.User = "root"; c.SSH.KeyPath = "k" },
		"dup sietch":    func(c *Config) { c.Sietches = []Sietch{{"A", PvE}, {"a", PvP}} },
		"bad name":      func(c *Config) { c.Sietches[0].Name = "x;rm -rf /" },
		"bad mode":      func(c *Config) { c.Sietches[0].Mode = "X" },
		"dd3 no role":   func(c *Config) { c.DeepDesert = DeepDesert{Instances: 3} },
		"dd range":      func(c *Config) { c.DeepDesert = DeepDesert{Instances: 4} },
		"dir injection": func(c *Config) { c.InstallDir = "/tmp/$(id)" },
		"dir relative":  func(c *Config) { c.InstallDir = "dune" },
		"title quote":   func(c *Config) { c.ServerTitle = `a"b` },
		"port":          func(c *Config) { c.WebPort = 70000 },
		"region":        func(c *Config) { c.Region = "Mars" },
		"too many":      func(c *Config) { c.Sietches = make([]Sietch, MaxSietches+1) },
		"distro inject": func(c *Config) { c.WSLDistro = "a;b" },
	}
	for name, mut := range cases {
		c := Default()
		mut(&c)
		if len(c.Validate()) == 0 {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestSecretsNotSerialised(t *testing.T) {
	c := Default()
	c.SSH.Password, c.SSH.Passphrase, c.FuncomToken = "pw1234", "pp1234", "tok1234"
	b := strings.ToLower(mustMarshal(c))
	for _, s := range []string{"pw1234", "pp1234", "tok1234"} {
		if strings.Contains(b, s) {
			t.Fatalf("secret %q leaked in JSON", s)
		}
	}
}

func TestConsoleAllowFrom(t *testing.T) {
	for _, ok := range []string{"", "203.0.113.7", "203.0.113.0/24", "2001:db8::/32"} {
		c := Default()
		c.ConsoleAllowFrom = ok
		if len(c.Validate()) != 0 {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"anywhere", "1.2.3", "1.2.3.4/99", "1.2.3.4; rm -rf /", "$(id)"} {
		c := Default()
		c.ConsoleAllowFrom = bad
		if len(c.Validate()) == 0 {
			t.Errorf("%q must be rejected", bad)
		}
	}
}
