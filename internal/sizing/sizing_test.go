package sizing

import (
	"testing"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
)

func host(gib int64, avx2 bool) probe.Info {
	return probe.Info{OS: "Linux", Arch: "x86_64", UID: 1000, AVX: avx2, AVX2: avx2, CPUThreads: 16,
		MemTotalKB: gib * 1024 * 1024, DiskFreeKB: 500 * 1024 * 1024, Tools: map[string]bool{}}
}

func has(r Report, code string, l Level) bool {
	for _, f := range r.Findings {
		if f.Code == code && f.Level == l {
			return true
		}
	}
	return false
}

func TestRequiredRAM(t *testing.T) {
	c := model.Default()
	if RequiredRAM(c) != 20 {
		t.Fatal("basic should be 20")
	}
	c.Sietches = append(c.Sietches, model.Sietch{Name: "B", Mode: model.PvE})
	c.DeepDesert.Instances = 1
	if got := RequiredRAM(c); got != 40 { // matches upstream "main + extra + Deep Desert = 40"
		t.Fatalf("got %d", got)
	}
}

func TestBlocking(t *testing.T) {
	r := Analyse(host(64, false), model.Default())
	if r.OK || !has(r, "avx2", Error) {
		t.Fatal("missing AVX2 must block")
	}
	r = Analyse(host(16, true), model.Default())
	if r.OK || !has(r, "ram-min", Error) {
		t.Fatal("16 GB must block")
	}
	h := host(64, true)
	h.UID = 0
	if Analyse(h, model.Default()).OK {
		t.Fatal("root must block")
	}
}

func TestLayoutWarningAndTuning(t *testing.T) {
	c := model.Default()
	c.Sietches = make([]model.Sietch, 4)
	r := Analyse(host(32, true), c)
	if !r.OK || !has(r, "ram-layout", Warn) {
		t.Fatalf("expected warning only: %+v", r.Findings)
	}
	if r.Env["DUNE_PG_JIT"] != "off" || len(r.PostCommands) == 0 {
		t.Fatalf("tuning missing: %+v", r)
	}
	if got := MaxSietchesFor(40); got != 3 {
		t.Fatalf("MaxSietchesFor(40)=%d", got)
	}
}

func TestWSLConfig(t *testing.T) {
	h := host(64, true)
	h.WSL = true
	if Analyse(h, model.Default()).WSLConfig == "" {
		t.Fatal("wslconfig expected")
	}
}
