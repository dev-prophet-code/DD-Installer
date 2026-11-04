# Roadmap

* End-to-end validation on Ubuntu 22.04/24.04 (SSH + local), Windows 11 + WSL2, macOS -> remote.
* Calibrate the RAM heuristic with real `dune memory status` data.
* Verify the per-Sietch PvP/PvE selectors against a running game; promote from *experimental*.
* Pin upstream release (`DUNE_RELEASE_SHA256`, tag selection) instead of "latest".
* SSH keyboard-interactive and jump-host support.
* Signed installers (MSI, notarised `.app`/dmg, AppImage); Linux arm64 build; per-OS native look polish.
* Verify WSL2 provisioning (UAC, reboot handling) on real Windows 10/11 machines.
* Resume-after-failure state, import of an existing install, Linux distro matrix in CI.
* Optional step for the Hyper-V Director travel-fix repository for Funcom's official Hyper-V servers.
* Localisation (English only for now).
