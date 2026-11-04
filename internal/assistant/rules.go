package assistant

import (
	"fmt"
	"regexp"
)

const docs = "https://docs.dunedocker.app/"

func re(p string) *regexp.Regexp { return regexp.MustCompile("(?i)" + p) }

// pkgInstall returns a script that installs packages with whichever package
// manager exists (mirrors the upstream installer's detection order).
func pkgInstall(pkgs map[string]string) string {
	// pkgs maps manager -> package list
	s := "set -e\nSUDO=; [ \"$(id -u)\" -eq 0 ] || SUDO=sudo\n"
	first := true
	for _, m := range []string{"apt-get", "dnf", "yum", "zypper", "pacman", "apk"} {
		p, ok := pkgs[m]
		if !ok {
			continue
		}
		kw := "elif"
		if first {
			kw, first = "if", false
		}
		cmd := ""
		switch m {
		case "apt-get":
			cmd = "$SUDO apt-get update && $SUDO apt-get install -y " + p
		case "dnf", "yum":
			cmd = "$SUDO " + m + " install -y " + p
		case "zypper":
			cmd = "$SUDO zypper --non-interactive install " + p
		case "pacman":
			cmd = "$SUDO pacman -Sy --noconfirm " + p
		case "apk":
			cmd = "$SUDO apk add --no-cache " + p
		}
		s += fmt.Sprintf("%s command -v %s >/dev/null 2>&1; then %s\n", kw, m, cmd)
	}
	s += "else echo 'No supported package manager found' >&2; exit 1; fi\n"
	return s
}

// Rules is the rule base. Order matters: specific rules first.
func Rules() []Rule {
	return []Rule{
		{
			ID: "docker-desktop-internal", Title: "Running inside Docker Desktop's internal Linux",
			Pattern: re(`docker desktop's internal linux|linuxkit`),
			Explain: "The installer ran inside Docker Desktop's own utility distribution. Dune Docker must be installed in your own Ubuntu WSL2 distribution (with Docker Desktop's WSL integration enabled) or in a Linux VM.",
			Risk:    RiskManual, DocURL: docs + "how-to/windows-wsl2",
			Actions: func(Context) []Action {
				return []Action{{Description: "Open the Windows installer, pick an Ubuntu WSL2 distribution (not docker-desktop) and enable Docker Desktop > Settings > Resources > WSL integration for it."}}
			},
		},
		{
			ID: "root-install", Title: "Installer must not run as root",
			Pattern: re(`must not be installed as root|run this as your normal linux user`),
			Explain: "Dune Docker refuses to install as root. Create a regular user with sudo access and connect as that user.",
			Risk:    RiskManual, DocURL: docs + "getting-started/installation",
			Actions: func(Context) []Action {
				return []Action{{Description: "On the server run: apt-get update && apt-get install -y sudo && adduser dune && usermod -aG sudo dune - then use user 'dune' in the installer."}}
			},
		},
		{
			ID: "sudo-password", Title: "sudo needs a password",
			Pattern: re(`sudo: a password is required|no tty present and no askpass|sudo: a terminal is required|this installer needs administrator access`),
			Explain: "The installer runs non-interactively and cannot type a sudo password. It can grant the install user temporary passwordless sudo using the password you entered; the grant is removed again when the installation finishes.",
			Risk:    RiskModerate, DocURL: docs + "getting-started/requirements",
			When: func(c Context) bool { return c.SudoPass },
			Actions: func(c Context) []Action {
				return []Action{{
					Description: "Create /etc/sudoers.d/90-dd-installer for user " + c.User + " (temporary, removed after installation).",
					Script:      "@sudo-temp-grant",
					Retry:       true,
				}}
			},
		},
		{
			ID: "sudo-password-manual", Title: "sudo needs a password",
			Pattern: re(`sudo: a password is required|no tty present and no askpass|sudo: a terminal is required|this installer needs administrator access`),
			When:    func(c Context) bool { return !c.SudoPass },
			Explain: "The installer cannot type a sudo password. Enter the account password in the SSH settings, or enable passwordless sudo for this user.",
			Risk:    RiskManual, DocURL: docs + "getting-started/requirements",
			Actions: func(c Context) []Action {
				return []Action{{Description: "On the server: echo '" + c.User + " ALL=(ALL) NOPASSWD:ALL' | sudo tee /etc/sudoers.d/90-dd-installer && sudo chmod 440 /etc/sudoers.d/90-dd-installer"}}
			},
		},
		{
			ID: "pkg-lock", Title: "Package manager is busy",
			Pattern: re(`could not get lock|unable to acquire the dpkg frontend lock|dpkg frontend lock|another process is using|waiting for cache lock|is another process using it`),
			Explain: "Another package operation (often automatic updates on a fresh server) holds the package lock. The installer waits for it to finish and retries.",
			Risk:    RiskSafe,
			Actions: func(Context) []Action {
				return []Action{{Description: "Wait up to 5 minutes for the package lock, then retry.", Wait: 60, Retry: true}}
			},
		},
		{
			ID: "missing-tools", Title: "Required tools are missing",
			Pattern: re(`(tar|curl|wget|curl or wget|python3|gpg|openssl) (is|are) (missing|required)|command not found.*(curl|tar|python3)|(curl|tar|python3): not found|required tools are still missing`),
			Explain: "curl, tar, openssl, gpg and python3 are prerequisites of the upstream installer. They can be installed with the system package manager.",
			Risk:    RiskSafe, DocURL: docs + "getting-started/installation",
			Actions: func(Context) []Action {
				return []Action{{
					Description: "Install curl, tar, openssl, python3 and gnupg with the package manager.",
					Script: pkgInstall(map[string]string{
						"apt-get": "ca-certificates curl bash tar openssl python3 gnupg",
						"dnf":     "ca-certificates curl bash tar openssl python3 gnupg2",
						"yum":     "ca-certificates curl bash tar openssl python3 gnupg2",
						"zypper":  "ca-certificates curl bash tar openssl python3 gpg2",
						"pacman":  "ca-certificates curl bash tar openssl python gnupg",
						"apk":     "ca-certificates curl bash tar openssl python3 gnupg",
					}),
					Retry: true,
				}}
			},
		},
		{
			ID: "install-dir-exists", Title: "Install folder already exists",
			Steps:   []string{"bootstrap"},
			Pattern: re(`already contains dune docker|already exists\. move or remove`),
			Explain: "The target folder already holds a Dune Docker installation (or a leftover folder). A fresh install must never overwrite an existing one; the installer will reuse a complete installation and continue with configuration instead.",
			Risk:    RiskSafe, DocURL: docs + "getting-started/installation",
			Actions: func(Context) []Action {
				return []Action{{Description: "Reuse the existing installation and continue.", Retry: true, Patch: &PlanPatch{ReuseExistingInstall: true}}}
			},
		},
		{
			ID: "no-space", Title: "Not enough disk space",
			Pattern: re(`no space left on device|has less than 100 mb free|cannot create .* (choose|check) free space|disk quota exceeded`),
			Explain: "The install volume (or Docker's storage, which can be on a different volume) is full. Project files and Docker storage are separate; moving the project folder alone does not free Docker's space.",
			Risk:    RiskManual, DocURL: docs + "how-to/install-on-another-drive",
			Actions: func(Context) []Action {
				return []Action{
					{Description: "Check where space is missing: df -hT; docker info --format '{{.DockerRootDir}}'; docker system df"},
					{Description: "Choose another install folder on a larger mounted volume in the installer, or enlarge the VM disk / LVM volume and filesystem."},
				}
			},
		},
		{
			ID: "avx", Title: "CPU does not provide AVX2",
			Pattern: re(`avx2? .*(not|unsupported|missing)|illegal instruction`),
			Explain: "The game server needs AVX/AVX2. On virtual machines the hypervisor must expose the host CPU features.",
			Risk:    RiskManual, DocURL: docs + "getting-started/requirements",
			Actions: func(c Context) []Action {
				hint := "Enable host CPU passthrough in your hypervisor (Proxmox: CPU type 'host'; VMware/Hyper-V/KVM: expose host CPU features)."
				return []Action{{Description: hint}}
			},
		},
		{
			ID: "docker-socket", Title: "No permission to use Docker",
			Pattern: re(`permission denied.*docker\.sock|got permission denied while trying to connect to the docker daemon|cannot connect to the docker daemon`),
			Explain: "The install user is not (yet) allowed to talk to the Docker daemon. Adding the user to the docker group fixes this; the installer then opens a fresh session so the membership applies. Never make the socket world-writable.",
			Risk:    RiskModerate, DocURL: docs + "operations/repair-permission-errors",
			Actions: func(Context) []Action {
				return []Action{{
					Description: "Add the install user to the 'docker' group (sudo usermod -aG docker), start Docker if stopped, reconnect and retry.",
					Script: `set -e
SUDO=; [ "$(id -u)" -eq 0 ] || SUDO=sudo
getent group docker >/dev/null 2>&1 || $SUDO groupadd docker
$SUDO usermod -aG docker "$(id -un)"
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then $SUDO systemctl enable --now docker || true; fi
`,
					Retry: true, Reconnect: true,
				}}
			},
		},
		{
			ID: "runtime-permissions", Title: "Runtime files have the wrong owner",
			Pattern: re(`not writable by|cannot update .*(\.env|runtime)|permission denied.*(runtime/|\.env)|eacces.*(runtime|\.env)|web container may not be able to write repo files`),
			Explain: "Project-managed runtime files are not writable by the install user, typically after a command was run with sudo. The upstream repair script restores ownership of the managed paths only.",
			Risk:    RiskSafe, DocURL: docs + "operations/repair-permission-errors",
			Actions: func(c Context) []Action {
				return []Action{{
					Description: "Run runtime/scripts/repair-host-runtime-permissions.sh in the install folder.",
					Script:      "set -e\ncd \"" + c.Dir + "\"\n@docker-group runtime/scripts/repair-host-runtime-permissions.sh\n",
					Retry:       true,
				}}
			},
		},
		{
			ID: "port-in-use", Title: "Port already in use",
			Pattern: re(`port \d+ is already in use|address already in use|port is already allocated|bind: address already in use`),
			Explain: "Another service uses the chosen web port. The installer picks the next free port and retries. Game ports (31982, 31983, UDP 7777-7810) must be free as well.",
			Risk:    RiskSafe, DocURL: docs + "how-to/change-game-ports",
			Actions: func(Context) []Action {
				return []Action{{Description: "Use the next free Web UI port and retry.", Retry: true, Patch: &PlanPatch{NextFreeWebPort: true}}}
			},
		},
		{
			ID: "steamcmd", Title: "SteamCMD could not download the game",
			Pattern: re(`steamcmd.*(fail|error|timeout|nxdomain|content)|failed to (download|install).*(game|depot)|app update.*fail|state is 0x\d+ after update job`),
			Explain: "Steam content hosts are sometimes unreachable or resolve badly (a documented past incident). Upstream ships `dune update fix-steamcmd`; the installer runs it and retries after a pause. Large downloads may also simply need more time.",
			Risk:    RiskSafe, DocURL: docs + "operations/troubleshooting",
			Actions: func(c Context) []Action {
				return []Action{{
					Description: "Run `dune update fix-steamcmd`, wait 2 minutes, retry.",
					Script:      "cd \"" + c.Dir + "\" && @docker-group runtime/scripts/dune update fix-steamcmd || true\n",
					Retry:       true, Wait: 120,
				}}
			},
		},
		{
			ID: "network-risk", Title: "Public-NAT bind risk detected",
			Pattern: re(`status=risk|ip_nonlocal_bind`),
			Explain: "`dune network status` reports a risky net.ipv4.ip_nonlocal_bind value for a NAT-hosted public server. The documented fix changes only that setting.",
			Risk:    RiskSafe, DocURL: docs + "operations/docker-desktop-wsl2-network-conflict",
			Actions: func(c Context) []Action {
				return []Action{{
					Description: "Run `dune network fix`.",
					Script:      "cd \"" + c.Dir + "\" && @docker-group runtime/scripts/dune network fix\n",
					Retry:       true,
				}}
			},
		},
		{
			ID: "console-down", Title: "Web console is not reachable",
			Steps:   []string{"console-setup"},
			Pattern: re(`dd_err console-not-reachable|connection refused|errno 111|remote end closed`),
			Explain: "The Console container did not answer on its port. The installer restarts it and checks its log before retrying.",
			Risk:    RiskSafe, DocURL: docs + "how-to/connection-problems",
			Actions: func(c Context) []Action {
				return []Action{{
					Description: "Restart the console container, show its last log lines and retry.",
					Script:      "cd \"" + c.Dir + "\" && @docker-group sh -c 'docker compose -f docker-compose.web.yml up -d redblink-dune-docker-console; sleep 10; docker logs --tail 40 redblink-dune-docker-console 2>&1' || true\n",
					Retry:       true, Wait: 15,
				}}
			},
		},
		{
			ID: "login-rate-limit", Title: "Console sign-in rate limit",
			Pattern: re(`too many sign-in attempts|http 429`),
			Explain: "The Console throttles sign-in attempts. The installer waits and tries again.",
			Risk:    RiskSafe,
			Actions: func(Context) []Action {
				return []Action{{Description: "Wait 5 minutes and retry.", Wait: 300, Retry: true}}
			},
		},
		{
			ID: "funcom-token", Title: "Funcom token rejected",
			Steps:   []string{"console-setup"},
			Pattern: re(`funcom.*(token|auth).*(invalid|mismatch|reject|empty)|dd_err token|token.*(invalid|expired)`),
			Explain: "The Funcom self-host service token is missing, expired or belongs to a different battlegroup. Generate a fresh token in your Funcom self-hosting account and enter it again.",
			Risk:    RiskManual, DocURL: docs + "getting-started/first-run",
			Actions: func(Context) []Action {
				return []Action{{Description: "Go back to 'Server' in the installer, paste a new Funcom token and press Install again (finished steps are skipped)."}}
			},
		},
		{
			ID: "github-unreachable", Title: "GitHub or the network is unreachable",
			Pattern: re(`github did not return|release download failed|could not resolve host|temporary failure in name resolution|connection timed out|tls|ssl.*(error|certificate)|curl: \(\d+\)`),
			Explain: "A download from GitHub, Steam or a container registry failed. This is usually transient (DNS, rate limit or firewall). The installer retries with a pause and shows DNS diagnostics.",
			Risk:    RiskSafe,
			Actions: func(Context) []Action {
				return []Action{{
					Description: "Check DNS/HTTPS reachability, wait 30 s and retry.",
					Script:      "getent hosts github.com || cat /etc/resolv.conf\ncurl -sS -m 15 -o /dev/null -w 'github.com: HTTP %{http_code}\\n' https://api.github.com/ || true\n",
					Retry:       true, Wait: 30,
				}}
			},
		},
		{
			ID: "wsl-systemd", Title: "systemd is disabled in WSL2",
			Pattern: re(`system has not been booted with systemd|failed to connect to bus`),
			When:    func(c Context) bool { return c.Host.WSL },
			Explain: "Scheduled restarts, backups and Docker's service need systemd. Enable it in /etc/wsl.conf and restart WSL (`wsl --shutdown` from Windows).",
			Risk:    RiskModerate, DocURL: docs + "how-to/windows-wsl2",
			Actions: func(Context) []Action {
				return []Action{{
					Description: "Write [boot] systemd=true to /etc/wsl.conf; then run `wsl --shutdown` in Windows and start the installer again.",
					Script:      "grep -q '^systemd *= *true' /etc/wsl.conf 2>/dev/null || printf '\\n[boot]\\nsystemd=true\\n' | sudo tee -a /etc/wsl.conf >/dev/null\n",
				}}
			},
		},
		{
			ID: "ssh-refused", Title: "SSH connection refused",
			Steps:   []string{"connect"},
			Pattern: re(`connection refused`),
			Explain: "Nothing accepts connections on that host/port. Check the port, that sshd is running (`sudo systemctl enable --now ssh`) and firewalls.",
			Risk:    RiskManual,
			Actions: func(Context) []Action {
				return []Action{{Description: "Verify host, port and that the OpenSSH server is installed and running."}}
			},
		},
		{
			ID: "ssh-timeout", Title: "SSH connection timed out",
			Steps:   []string{"connect"},
			Pattern: re(`i/o timeout|no route to host|network is unreachable`),
			Explain: "The server is not reachable from this computer (wrong IP, VPN needed, or a firewall/security group blocks TCP 22).",
			Risk:    RiskManual,
			Actions: func(Context) []Action {
				return []Action{{Description: "Check the IP/hostname, VPN and the provider's firewall rules for the SSH port."}}
			},
		},
		{
			ID: "ssh-auth", Title: "SSH authentication failed",
			Steps:   []string{"connect"},
			Pattern: re(`unable to authenticate|permission denied \(|no supported methods remain|passphrase`),
			Explain: "The server rejected the credentials. Check user name, key file (and passphrase) or password; many servers disable password login.",
			Risk:    RiskManual,
			Actions: func(Context) []Action {
				return []Action{{Description: "Use the key authorised in ~/.ssh/authorized_keys of that user, or enable PasswordAuthentication in sshd_config."}}
			},
		},
	}
}
