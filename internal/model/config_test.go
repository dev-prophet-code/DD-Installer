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
