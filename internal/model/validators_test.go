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
