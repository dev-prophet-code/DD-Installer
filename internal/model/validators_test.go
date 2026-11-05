package model

import "testing"

func TestFieldValidators(t *testing.T) {
	ok := map[string][]string{
		"sietch": {"Sietch 1", "Harko_Outpost", "a"},
		"title":  {"My Dune Server", "x"},
		"port":   {"22", "8088", "65535"},
		"host":   {"example.com", "203.0.113.10", "my-alias", "[::1]", "2001:db8::1"},
		"user":   {"dune", "dd.user", "a-b_c"},
	}
	bad := map[string][]string{
		"sietch": {"", "-x", "a;b", "01234567890123456789012345678901234"},
		"title":  {"", "  ", "a\"b", "x$y"},
		"port":   {"", "0", "65536", "22abc", "-1", "08088"},
		"host":   {"", "a b", "a;b", "bad/host"},
		"user":   {"", "root", "a b", "x;y"},
	}
	fns := map[string]func(string) error{"sietch": ValidSietchName, "title": ValidTitle, "port": ValidPort, "host": ValidHost, "user": ValidUser}
	for k, vals := range ok {
		for _, v := range vals {
			if err := fns[k](v); err != nil {
				t.Errorf("%s %q should be valid: %v", k, v, err)
			}
		}
	}
	for k, vals := range bad {
		for _, v := range vals {
			if fns[k](v) == nil {
				t.Errorf("%s %q should be invalid", k, v)
			}
		}
	}
	if ValidAllowFrom("") != nil || ValidAllowFrom("10.0.0.0/8") != nil || ValidAllowFrom("nope") == nil {
		t.Error("allow-from validator")
	}
}

// Validate is the last line of defence: SSH user and host end up in scripts
// and in /etc/sudoers.d, so values that bypass the UI fields are rejected too.
func TestValidateRejectsHostileSSHUserAndHost(t *testing.T) {
	bad := []string{"x'; touch /tmp/p; echo '", "dune\nmallory", "dune\n", "a b", "$(id)", "`id`", "dune ALL=(ALL) ALL", "root", " "}
	for _, u := range bad {
		c := Default()
		c.Target = TargetSSH
		c.SSH = SSHConfig{Host: "h", Port: 22, User: u, Auth: AuthAgent}
		if len(c.Validate()) == 0 {
			t.Errorf("user %q accepted", u)
		}
	}
	for _, h := range []string{"evil host", "h\nname", "h;id", "h$(id)", "h\n"} {
		c := Default()
		c.Target = TargetSSH
		c.SSH = SSHConfig{Host: h, Port: 22, User: "dune", Auth: AuthAgent}
		if len(c.Validate()) == 0 {
			t.Errorf("host %q accepted", h)
		}
	}
	c := Default()
	c.Target = TargetSSH
	c.SSH = SSHConfig{Host: "203.0.113.10", Port: 22, User: "dune.user-1", Auth: AuthAgent}
	if errs := c.Validate(); len(errs) != 0 {
		t.Fatalf("a normal user/host was rejected: %v", errs)
	}
}
