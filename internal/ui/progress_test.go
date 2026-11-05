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

func FuzzParseProgress(f *testing.F) {
	f.Add("progress: 45.12")
	f.Add("100%")
	f.Add("progress: 1e9 %")
	f.Fuzz(func(t *testing.T, in string) {
		if pct, ok := parseProgress(in); ok && (pct < 0 || pct > 100) {
			t.Fatalf("percentage out of range: %v", pct)
		}
	})
}
