package windows

// Pure helpers for installing WSL2 + Ubuntu. They have no build tag so they
// can be unit-tested on any OS; the code that actually calls wsl.exe lives in
// windows.go (build tag windows).

import (
	"strings"
)

// ProvisionUser is the Linux account created in a freshly installed Ubuntu.
const ProvisionUser = "dune"

// exitRebootRequired is ERROR_SUCCESS_REBOOT_REQUIRED.
const exitRebootRequired = 3010

// pickUbuntu returns the first Ubuntu distribution from a wsl -l -q listing.
func pickUbuntu(distros []string) string {
	for _, d := range distros {
		if strings.HasPrefix(strings.ToLower(d), "ubuntu") {
			return d
		}
	}
	return ""
}

// installCommand is the elevated (UAC) PowerShell command that installs
// WSL2 and Ubuntu without launching the interactive first-run prompt.
// Elevation output cannot be captured, so the exit code is propagated.
func installCommand() []string {
	return []string{
		"powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"$p = Start-Process -FilePath wsl.exe -ArgumentList '--install','-d','Ubuntu','--no-launch' -Verb RunAs -Wait -PassThru; exit $p.ExitCode",
	}
}

// rebootNeeded decides from the exit code and the distro list after the
// install whether Windows has to restart before WSL can be used.
func rebootNeeded(exit int, distrosAfter []string) bool {
	if exit == exitRebootRequired {
		return true
	}
	// Exit 0 but no distribution yet: the WSL feature was just enabled and
	// the distribution registers after the next boot.
	return exit == 0 && pickUbuntu(distrosAfter) == ""
}

// provisionScript runs as root inside the new distribution. It creates the
// service user, grants it passwordless sudo (required because the installer
// runs unattended; this is a dedicated distribution) and enables systemd,
// which the upstream installer needs for Docker and scheduled tasks.
func provisionScript() string {
	return `set -e
U=` + ProvisionUser + `
id "$U" >/dev/null 2>&1 || useradd -m -s /bin/bash -G sudo "$U"
printf '%s ALL=(ALL) NOPASSWD:ALL\n' "$U" > /etc/sudoers.d/90-dd-installer-wsl
chmod 440 /etc/sudoers.d/90-dd-installer-wsl
if command -v visudo >/dev/null 2>&1; then visudo -cf /etc/sudoers.d/90-dd-installer-wsl >/dev/null; fi
if [ ! -s /etc/wsl.conf ]; then
  printf '[boot]\nsystemd=true\n\n[user]\ndefault=%s\n' "$U" > /etc/wsl.conf
else
  grep -q '^systemd *= *true' /etc/wsl.conf || printf '\n[boot]\nsystemd=true\n' >> /etc/wsl.conf
  grep -q '^default *=' /etc/wsl.conf || printf '\n[user]\ndefault=%s\n' "$U" >> /etc/wsl.conf
fi
echo "Ubuntu prepared: user $U, systemd enabled."
`
}

// provisionHint explains what the installer will do (shown before consent).
const provisionHint = "WSL2 with Ubuntu is not installed. The installer can install it for you: " +
	"Windows asks for administrator permission (UAC), then Ubuntu is set up with a user named '" + ProvisionUser +
	"' (passwordless sudo, systemd enabled). A restart of Windows may be required; after it, start the installer again."
