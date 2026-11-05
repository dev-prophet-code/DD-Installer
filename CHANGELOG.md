# Changelog

Versions follow the number in the [`VERSION`](VERSION) file. Newest first.

## 0.0.5-alpha

Security release: findings of an internal code review and penetration test are fixed.

**Security**
* **Verified downloads.** Before every download DD-Installer reads the current checksum list (`upstream/upstream.json`, refreshed daily by a workflow; the tested release is only changed by hand) and checks the SHA-256 of all downloaded files **before** anything is run or moved into place. No checksum, a mismatch or an archive containing links stops the installation. The built-in pin is the offline fallback. "Newest upstream" uses the same verified path; upstream's `bootstrap.sh` is no longer piped into a shell.
* **Temporary sudo access** is now removed **immediately** after a final check of the installation (files, web console, battlegroup, Docker access), and the removal is verified. A systemd unit also removes it at the next boot, because the 3-hour timer alone did not survive a reboot. The user name is validated and quoted before it goes into the sudoers file.
* **SSH host keys:** keys marked `@revoked` in `~/.ssh/known_hosts` are always refused and cannot be confirmed. A wrong password on a trusted host is reported as a login error instead of a host-key question.
* **Input validation:** the strict user and host checks of the input fields now apply to every value, including those imported from `~/.ssh/config`.
* **Console traffic:** the helper on the server never sends the admin password or the Funcom token through an HTTP proxy from the environment (it checks for proxy settings and connects directly).
* **Admin password on the server:** the folder with the install log is private and is deleted as soon as the step has finished.
* **Uninstall guard:** the folder is resolved first (symlinks, `..`, `//`); the home directory and every folder that contains it are always refused.
* **New notice before installing** about the unencrypted web console (plain HTTP). It adapts to your settings and must be acknowledged.

**Other**
* Builds use Go 1.26 (required by updated dependencies); Linux builds need the Wayland development packages.
* New tests for every fix above (real shell, real SSH server, GUI flow).
* README and release notes now say that the alpha was tested on real servers and desktop systems and that problems can still occur.

## 0.0.4-alpha

**Reliability**
* The long install step (Docker, web console) now runs **detached on the server** and is only followed by DD-Installer. A dropped SSH connection, a sleeping laptop or a closed window no longer stops it; the app reconnects (with back-off) and continues reading the log. Cancel really stops the job on the server.
* **Resume:** pressing Install again continues where it stopped - finished steps (download, console, deployment, start, readiness) are detected on the server and skipped; a running deployment is re-attached instead of started twice.
* **Pinned upstream:** Dune Docker `v1.4.61` (the tested version) is installed by default, with optional SHA-256 verification; "newest upstream" is an option. A version mismatch is reported. `scripts/update-upstream-pin.sh` updates the pin.
* The computer is **kept awake** while installing (Windows, macOS, Linux).
* The temporary sudo grant also **removes itself after 3 hours**.
* Funcom **token check** with paste button and live feedback.

**New features**
* **Welcome page** (what you need / what will happen) and **Finish page** (console link, admin password with reveal/copy, port checklist, health check, "Start over" clears secrets).
* **Manage existing servers:** status, backup, repair permissions, update checks, game and console updates, restart, stop, and "remove everything" (typed confirmation, guard rails). Also the **Hyper-V Director travel fix** for Funcom's official Hyper-V server (pinned commit).
* **Firewall helper:** opens the game ports in an active ufw/firewalld (only adds rules, SSH first, never enables/resets), optional restriction of the web console to your IP.
* **Verification step** prints Sietches, Deep Desert layout and ports after setup.
* **SSH:** `~/.ssh/config` import (Host, User, Port, IdentityFile, ProxyJump), single **jump host**, trusted keys from `~/.ssh/known_hosts` (conflicts are errors), servers that only offer keyboard-interactive passwords, **Windows OpenSSH agent**.
* **UI:** inline field validation, overall and download progress bars, monospace log with filter and follow, **effects on/off** switch (saved; `DD_INSTALLER_NO_EFFECTS=1`), animated demo in the README.

**Quality**
* New tests: controller, in-process SSH server, real-shell tests of the detached runner, firewall/uninstall guard rails; fuzz tests for all parsers. Fuzzing found and fixed: quadratic line splitting on progress output, a race in detached polling, a UI freeze on heavy log output, `Match` blocks in ssh config.
* `ui.go` split into one file per page; the disclaimer has one source (`internal/app/disclaimer.md`).

**Build and project**
* Release workflow: Apple-Silicon runner `macos-14`, create-or-update instead of delete/recreate, release notes from this file, build provenance attestation, separate **Release macOS** workflow to add a missing macOS build.
* CI: race detector, govulncheck and golangci-lint (advisory at first), screenshot artifact; actions pinned to commit SHAs; Dependabot.
* New docs: FAQ, router & firewall guide, hosting provider notes, troubleshooting. Security policy, contributing guide, issue and PR templates.

## 0.0.3-alpha

* New Dune: Awakening inspired look and feel: Arrakis-night theme, embedded fonts (Rajdhani, Cinzel Decorative), generated desert backdrop with moons and dunes, drifting sand, glow and shimmer effects, page transitions, sandstorm during installation. Identical on Windows, macOS and Linux.
* SSH made easy: in-app **Create SSH key** (Ed25519, owner-only permissions, never overwrites) with copy-paste commands for Windows/macOS/Linux, in-app **SSH help**, and a beginner guide ([docs/SSH-GUIDE.md](docs/SSH-GUIDE.md)).
* Install page reorganised: summary and start on top, progress and assistant on the left, live log on the right.
* Program logo: used as window icon and in the header on all systems, in the README, as `DD-Installer.app` icon on macOS and as menu icon on Linux (`install-desktop.sh`). The Windows `.exe` carries the logo, version info and a DPI-aware manifest (generated in the release run with go-winres, `platforms/windows/winres`).
* README shows screenshots of every step (`docs/images`, generated by `TestShots`).
* Fix (Windows): no more briefly flashing console windows. Background calls to `wsl.exe`/`powershell.exe` are started without a window, and the WSL distribution list is cached for a few seconds instead of being queried on every click.
* Fix: pages are re-laid out when the wizard changes step.
* Docs: new design documentation and font credits; disclaimer mentions the key helper.

## 0.0.2-alpha

* Native desktop application (Fyne) for Windows, macOS and Linux; the web interface was removed.
* Windows: detects WSL2/Ubuntu, offers to install them (UAC), prepares a user with systemd and continues the setup.
* Versioned releases built automatically per operating system (CI).
* Disclaimer shown in the app and required before installing.
* Review and self-conducted penetration test fixes (stdin-safe script execution, shell-injection tests, assistant retry logic, event delivery).

## 0.0.1-alpha

* First draft: Go core (model, scan, sizing, problem-solving assistant, SSH/local transport), temporary local web UI, documentation, per-OS folders.
