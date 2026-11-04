// Package sizing turns a probed HostInfo plus the user's wishes into
// blocking errors, warnings and tuning recommendations.
//
// Hard requirements come from the upstream docs (Red-Blink/dune-docker-docs,
// getting-started/requirements.md): AVX/AVX2, >= 20 GB RAM, >= 200 GB disk,
// non-root user with sudo. The per-map memory costs below are a
// *conservative heuristic* derived from the upstream layout table
// (20 GB basic / 30 GB extra maps / 40 GB with Deep Desert) and the note that
// one Survival server idles at roughly 10-12 GB. Calibrate them against
// `dune memory status` on real hosts (see docs/SIZING.md).
package sizing

import (
	"fmt"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
)

type Level string

const (
	Error Level = "error" // blocks installation
	Warn  Level = "warn"
	Info  Level = "info"
)

type Finding struct {
	Level   Level  `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Heuristic constants (GiB).
const (
	BaseGiB          = 20 // core services + primary Hagga Basin + social maps
	PerExtraSietch   = 10 // one more Survival instance (~10-12 GB resident)
	PerDeepDesert    = 10 // upstream: 30 -> 40 GB when Deep Desert is added
	MinDiskGiB       = 200
	HeavyActivityGiB = 60
)

type Report struct {
	Findings       []Finding         `json:"findings"`
	RequiredRAMGiB int               `json:"requiredRamGiB"`
	MaxSietches    int               `json:"maxSietches"` // that fit in RAM with no Deep Desert
	Env            map[string]string `json:"env"`         // suggested .env overrides
	PostCommands   []string          `json:"postCommands"`
	WSLConfig      string            `json:"wslConfig,omitempty"`
	OK             bool              `json:"ok"`
}

// RequiredRAM estimates the RAM needed for the requested layout.
func RequiredRAM(cfg model.Config) int {
	need := BaseGiB
	if n := len(cfg.Sietches); n > 1 {
		need += (n - 1) * PerExtraSietch
	}
	need += cfg.DeepDesert.Instances * PerDeepDesert
	return need
}

// MaxSietchesFor returns how many Sietches fit into totalGiB of RAM.
func MaxSietchesFor(totalGiB float64) int {
	if totalGiB < BaseGiB {
		return 0
	}
	return 1 + int((totalGiB-BaseGiB)/PerExtraSietch)
}

// Analyse evaluates a probed host against a plan.
func Analyse(h probe.Info, cfg model.Config) Report {
	r := Report{Env: map[string]string{}, RequiredRAMGiB: RequiredRAM(cfg)}
	add := func(l Level, code, f string, a ...any) {
		r.Findings = append(r.Findings, Finding{l, code, fmt.Sprintf(f, a...)})
	}

	// --- hard requirements -------------------------------------------------
	if h.OS != "" && h.OS != "Linux" {
		add(Error, "os", "Target reports %q; Dune Docker needs a Linux host.", h.OS)
	}
	if h.Arch != "" && h.Arch != "x86_64" && h.Arch != "amd64" {
		add(Error, "arch", "Target architecture is %s; the Dune server images are x86-64 only.", h.Arch)
	}
	if !h.AVX2 {
		if h.AVX {
			add(Error, "avx2", "CPU/VM exposes AVX but not AVX2, which the game server requires.")
		} else {
			add(Error, "avx2", "CPU/VM does not expose AVX/AVX2 (in a VM enable host-CPU passthrough).")
		}
	}
	if h.UID == 0 {
		add(Error, "root", "Connected as root. Dune Docker must be installed by a regular user with sudo.")
	}
	if h.SudoNoPasswd != nil && !*h.SudoNoPasswd {
		add(Warn, "sudo", "sudo asks for a password; the installer will supply the SSH/sudo password if one was entered.")
	}
	if h.WSL && !h.Systemd {
		add(Warn, "wsl-systemd", "WSL2 without systemd: scheduled restarts/backups that use systemd timers may not work. Enable systemd in /etc/wsl.conf.")
	}
	if h.MemTotalKB > 0 {
		total := h.MemTotalGiB()
		switch {
		case total < BaseGiB-1: // reported MemTotal is slightly below nominal
			add(Error, "ram-min", "%.1f GB RAM detected; at least 20 GB is required.", total)
		case total < float64(r.RequiredRAMGiB)-1:
			add(Warn, "ram-layout", "Requested layout needs about %d GB RAM but only %.1f GB is available. Reduce Sietches/Deep Desert or add RAM.", r.RequiredRAMGiB, total)
		}
		r.MaxSietches = MaxSietchesFor(total + 1)
		if total >= float64(r.RequiredRAMGiB)-1 && total < float64(r.RequiredRAMGiB)+4 {
			add(Info, "ram-tight", "RAM is close to the estimate for this layout; leave headroom for backups and updates.")
		}
	}
	if h.DiskFreeKB > 0 && h.DiskFreeGiB() < MinDiskGiB {
		add(Warn, "disk", "%.0f GB free on the install volume; 200 GB or more is recommended (game, images, logs, backups).", h.DiskFreeGiB())
	}
	if h.CPUThreads > 0 && h.CPUThreads < 4 {
		add(Warn, "cpu", "Only %d CPU threads; expect slow map startup and tick rates.", h.CPUThreads)
	}
	for _, p := range h.BusyPorts {
		switch p {
		case cfg.WebPort:
			add(Warn, "port-web", "Port %d is already in use; the installer will pick the next free port.", p)
		case 31982, 31983, 15432:
			add(Error, "port-busy", "Port %d is already in use by another service.", p)
		}
	}
	for _, t := range []string{"curl", "tar"} {
		if ok, seen := h.Tools[t]; seen && !ok {
			add(Warn, "tool-"+t, "%s is missing; the installer will try to install it with the package manager.", t)
		}
	}
	if !h.Docker {
		add(Info, "docker", "Docker is not installed; the upstream installer will install it.")
	} else if !h.Compose {
		add(Warn, "compose", "Docker is present but the compose plugin is missing; the upstream installer will try to add it.")
	}

	// --- tuning recommendations -------------------------------------------
	if h.MemTotalKB > 0 {
		total := h.MemTotalGiB()
		r.Env["DUNE_PG_SHM_SIZE"] = pgShm(total)
		r.Env["DUNE_PG_JIT"] = "off"
		if total < float64(r.RequiredRAMGiB) && h.SwapTotalKB == 0 {
			r.PostCommands = append(r.PostCommands, "runtime/scripts/dune memory-swap enable")
			add(Info, "swap", "No swap and RAM below the layout estimate: a bounded memory-swap safety pool will be enabled (not a replacement for RAM).")
		}
	}
	// PostgreSQL on SSD/NVMe-like storage benefits from a lower random_page_cost.
	r.Env["DUNE_PG_RANDOM_PAGE_COST"] = "1.1"
	if h.CPUThreads > 0 && h.CPUThreads <= 8 {
		// Fewer RabbitMQ schedulers on small hosts so game servers keep the cores.
		r.Env["DUNE_RMQ_ERL_ARGS"] = "+S 2:2 +sbwt none +sbwtdcpu none +sbwtdio none"
	}

	if h.WSL || cfg.Target == model.TargetLocal && h.Kernel != "" && containsFold(h.Kernel, "microsoft") {
		// WSL2 caps memory at 50% of host RAM by default; surface a .wslconfig.
		r.WSLConfig = fmt.Sprintf("[wsl2]\nmemory=%dGB\nswap=8GB\n", maxInt(r.RequiredRAMGiB, 20))
		add(Info, "wslconfig", "WSL2 limits memory by default. Add the suggested .wslconfig on Windows (see docs) and run `wsl --shutdown`.")
	}

	r.OK = true
	for _, f := range r.Findings {
		if f.Level == Error {
			r.OK = false
		}
	}
	return r
}

func pgShm(totalGiB float64) string {
	switch {
	case totalGiB >= 40:
		return "1g"
	case totalGiB >= 30:
		return "768m"
	default:
		return "512m"
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
