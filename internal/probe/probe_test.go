package probe

import "testing"

func TestParse(t *testing.T) {
	out := "os=Linux\narch=x86_64\nuid=1000\ncpu_threads=16\navx=1\navx2=1\nmem_total_kb=33554432\nswap_total_kb=0\ndisk_free_kb=419430400\nwsl=0\nhas_curl=1\nhas_tar=0\ndocker=1\ncompose=0\nsudo_nopasswd=0\nport_busy_8088=1\ngarbage\n"
	i := Parse(out)
	if i.OS != "Linux" || i.CPUThreads != 16 || !i.AVX2 || i.UID != 1000 {
		t.Fatalf("bad parse: %+v", i)
	}
	if i.MemTotalGiB() != 32 {
		t.Fatalf("mem = %v", i.MemTotalGiB())
	}
	if !i.Tools["curl"] || i.Tools["tar"] {
		t.Fatal("tools")
	}
	if i.SudoNoPasswd == nil || *i.SudoNoPasswd {
		t.Fatal("sudo")
	}
	if len(i.BusyPorts) != 1 || i.BusyPorts[0] != 8088 {
		t.Fatal("ports")
	}
}
