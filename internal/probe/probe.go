// Package probe scans a target machine. A single POSIX shell script runs on
// the target (locally, over SSH, or inside WSL2) and prints key=value lines
// that Parse turns into HostInfo. Keeping the scan in one script means all
// three transports behave identically.
package probe

import (
	"bufio"
	"strconv"
	"strings"
)

// Script is executed with `sh -s` on the target. It is read-only.
const Script = `
export LC_ALL=C
echo "os=$(uname -s 2>/dev/null)"
echo "arch=$(uname -m 2>/dev/null)"
echo "kernel=$(uname -r 2>/dev/null)"
[ -r /etc/os-release ] && . /etc/os-release && echo "distro=${PRETTY_NAME:-$NAME}" && echo "distro_id=${ID:-}"
echo "user=$(id -un 2>/dev/null)"
echo "uid=$(id -u 2>/dev/null)"
echo "home=$HOME"
if [ -r /proc/cpuinfo ]; then
  echo "cpu_model=$(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ *//')"
  flags=$(grep -m1 '^flags' /proc/cpuinfo)
  case "$flags" in *" avx "*) echo "avx=1";; *) echo "avx=0";; esac
  case "$flags" in *" avx2 "*) echo "avx2=1";; *) echo "avx2=0";; esac
fi
echo "cpu_threads=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null)"
if [ -r /proc/meminfo ]; then
  echo "mem_total_kb=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)"
  echo "mem_avail_kb=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)"
  echo "swap_total_kb=$(awk '/^SwapTotal:/{print $2}' /proc/meminfo)"
fi
echo "disk_free_kb=$(df -Pk "${DUNE_PROBE_DIR:-$HOME}" 2>/dev/null | awk 'NR==2{print $4}')"
echo "disk_total_kb=$(df -Pk "${DUNE_PROBE_DIR:-$HOME}" 2>/dev/null | awk 'NR==2{print $2}')"
if grep -qi microsoft /proc/version 2>/dev/null; then echo "wsl=1"; else echo "wsl=0"; fi
[ -d /run/systemd/system ] && echo "systemd=1" || echo "systemd=0"
command -v systemd-detect-virt >/dev/null 2>&1 && echo "virt=$(systemd-detect-virt 2>/dev/null)"
for t in curl tar openssl python3 gpg sudo; do
  command -v $t >/dev/null 2>&1 && echo "has_$t=1" || echo "has_$t=0"
done
if command -v docker >/dev/null 2>&1; then
  echo "docker=1"
  echo "docker_version=$(docker version --format '{{.Server.Version}}' 2>/dev/null)"
  docker compose version >/dev/null 2>&1 && echo "compose=1" || echo "compose=0"
else
  echo "docker=0"
fi
if [ "$(id -u 2>/dev/null)" != "0" ] && command -v sudo >/dev/null 2>&1; then
  sudo -n true 2>/dev/null && echo "sudo_nopasswd=1" || echo "sudo_nopasswd=0"
fi
for p in 8088 31982 31983 15432; do
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | awk '{print $4}' | grep -q ":$p\$" && echo "port_busy_$p=1"
  fi
done
true
`

// Info is the parsed scan result.
type Info struct {
	OS, Arch, Kernel string
	Distro, DistroID string
	User, Home       string
	UID              int
	CPUModel         string
	CPUThreads       int
	AVX, AVX2        bool
	MemTotalKB       int64
	MemAvailKB       int64
	SwapTotalKB      int64
	DiskFreeKB       int64
	DiskTotalKB      int64
	WSL              bool
	Systemd          bool
	Virt             string
	Tools            map[string]bool
	Docker           bool
	DockerVersion    string
	Compose          bool
	SudoNoPasswd     *bool // nil = not applicable / unknown
	BusyPorts        []int
	Raw              map[string]string
}

func (i Info) MemTotalGiB() float64 { return float64(i.MemTotalKB) / 1024 / 1024 }
func (i Info) DiskFreeGiB() float64 { return float64(i.DiskFreeKB) / 1024 / 1024 }

// Parse reads the key=value output of Script. Unknown keys are kept in Raw.
func Parse(out string) Info {
	info := Info{Tools: map[string]bool{}, Raw: map[string]string{}}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		info.Raw[k] = v
		n, _ := strconv.ParseInt(v, 10, 64)
		switch {
		case k == "os":
			info.OS = v
		case k == "arch":
			info.Arch = v
		case k == "kernel":
			info.Kernel = v
		case k == "distro":
			info.Distro = v
		case k == "distro_id":
			info.DistroID = v
		case k == "user":
			info.User = v
		case k == "home":
			info.Home = v
		case k == "uid":
			info.UID = int(n)
		case k == "cpu_model":
			info.CPUModel = v
		case k == "cpu_threads":
			info.CPUThreads = int(n)
		case k == "avx":
			info.AVX = v == "1"
		case k == "avx2":
			info.AVX2 = v == "1"
		case k == "mem_total_kb":
			info.MemTotalKB = n
		case k == "mem_avail_kb":
			info.MemAvailKB = n
		case k == "swap_total_kb":
			info.SwapTotalKB = n
		case k == "disk_free_kb":
			info.DiskFreeKB = n
		case k == "disk_total_kb":
			info.DiskTotalKB = n
		case k == "wsl":
			info.WSL = v == "1"
		case k == "systemd":
			info.Systemd = v == "1"
		case k == "virt":
			info.Virt = v
		case strings.HasPrefix(k, "has_"):
			info.Tools[strings.TrimPrefix(k, "has_")] = v == "1"
		case k == "docker":
			info.Docker = v == "1"
		case k == "docker_version":
			info.DockerVersion = v
		case k == "compose":
			info.Compose = v == "1"
		case k == "sudo_nopasswd":
			b := v == "1"
			info.SudoNoPasswd = &b
		case strings.HasPrefix(k, "port_busy_") && v == "1":
			p, _ := strconv.Atoi(strings.TrimPrefix(k, "port_busy_"))
			info.BusyPorts = append(info.BusyPorts, p)
		}
	}
	return info
}
