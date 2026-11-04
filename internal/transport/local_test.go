package transport

import (
	"context"
	"strings"
	"testing"
)

// A child process that reads stdin must not swallow the rest of the script.
func TestChildCannotEatScript(t *testing.T) {
	var b strings.Builder
	code, err := NewLocal().Run(context.Background(), "cat >/dev/null\necho AFTER_CHILD\nexit 3\n", &b)
	if err != nil || code != 3 || !strings.Contains(b.String(), "AFTER_CHILD") {
		t.Fatalf("code=%d err=%v out=%q", code, err, b.String())
	}
}
