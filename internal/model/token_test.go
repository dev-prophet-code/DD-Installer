package model

import (
	"strings"
	"testing"
)

func TestCheckToken(t *testing.T) {
	good := strings.Repeat("a1B2", 12)
	if tok, p, w := CheckToken("  " + good + "\n"); p != "" || w != "" || tok != good {
		t.Fatalf("good token: %q %q %q", tok, p, w)
	}
	for _, bad := range []string{"", "   ", "abc def", good + "\n" + good, "tok\u00e9n" + good, "\"" + good + "\"", good + "$(x)"} {
		if _, p, _ := CheckToken(bad); p == "" {
			t.Errorf("%q must be rejected", bad)
		}
	}
	if _, p, w := CheckToken("shorttoken"); p != "" || w == "" {
		t.Fatal("short token: warning only")
	}
}
