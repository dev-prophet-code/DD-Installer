package ui

import "testing"

func TestParseProgress(t *testing.T) {
	cases := []struct {
		in  string
		pct float64
		ok  bool
	}{
		{"Update state (0x61) downloading, progress: 45.12 (1234 / 2735)", 45.12, true},
		{"Update state (0x81) verifying update, progress: 100.00", 100, true},
		{"downloading 37% done", 37, true},
		{"progress: 250", 0, false},
		{"nothing here", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseProgress(c.in)
		if ok != c.ok || (ok && got != c.pct) {
			t.Errorf("%q -> %v %v, want %v %v", c.in, got, ok, c.pct, c.ok)
		}
	}
}
