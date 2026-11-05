package probe

import "testing"

// Whatever a (possibly broken or hostile) target prints, parsing must not panic.
func FuzzParse(f *testing.F) {
	f.Add("os=Linux\nmem_total_kb=1024\navx2=1\nport_busy_8088=1\n")
	f.Add("=\n==\nmem_total_kb=\nport_busy_x=1\nsudo_nopasswd=0")
	f.Add("mem_total_kb=99999999999999999999\ncpu_threads=-1\n")
	f.Fuzz(func(t *testing.T, in string) {
		i := Parse(in)
		_ = i.MemTotalGiB()
		_ = i.DiskFreeGiB()
		if i.Tools == nil || i.Raw == nil {
			t.Fatal("maps must always be initialised")
		}
	})
}
