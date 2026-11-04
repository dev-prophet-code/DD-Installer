package windows

import (
	"testing"
	"time"
)

func TestListCache(t *testing.T) {
	now := time.Unix(1000, 0)
	c := newListCache(5 * time.Second)
	c.now = func() time.Time { return now }
	calls := 0
	load := func() []string { calls++; return []string{"Ubuntu"} }

	c.get(load)
	c.get(load)
	if calls != 1 {
		t.Fatalf("second call within the TTL must not spawn a process again (calls=%d)", calls)
	}
	now = now.Add(6 * time.Second)
	c.get(load)
	if calls != 2 {
		t.Fatalf("expired entry must reload (calls=%d)", calls)
	}
	c.invalidate()
	c.get(load)
	if calls != 3 {
		t.Fatalf("invalidate must force a reload (calls=%d)", calls)
	}
	got := c.get(load)
	got[0] = "changed"
	if c.get(load)[0] != "Ubuntu" {
		t.Fatal("callers must not be able to modify the cached list")
	}
}
