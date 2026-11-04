# Windows

* Code: `windows.go` (build tag `windows`).
* **Local install** runs inside an **Ubuntu WSL2 distribution** via `wsl.exe -d <distro> -- sh -s`. Docker Desktop's internal `docker-desktop` distributions are never used (upstream forbids it).
* **WSL2 is checked automatically.** If WSL2 or Ubuntu is missing, the installer offers to install them (UAC prompt), creates a user `dune`, enables systemd and then continues. A Windows restart may be needed; start the installer again afterwards. Docker Desktop is not required - the upstream installer installs Docker Engine inside Ubuntu.
* `provision.go` holds the testable logic (no build tag), `windows.go` the `wsl.exe` calls.
* WSL2 limits memory (default 50%). The scan suggests a `%UserProfile%\.wslconfig`; run `wsl --shutdown` afterwards.
* **Remote install** uses the built-in Go SSH client - no OpenSSH or PuTTY required.
* Build on Windows (MinGW-w64 gcc needed for the native GUI): `bash scripts/build.sh` (uses `-H=windowsgui`, no console window). Signed MSI is on the roadmap (unsigned exe triggers SmartScreen).
* Icon: the logo is the window and taskbar icon while the app runs. The release build embeds it into the `.exe` (icon, version info, DPI-aware manifest) with `winres/make-resources.sh` (go-winres, run only on the GitHub runner).
* Look: the Dune-inspired theme is drawn by the toolkit and is identical to the other systems. High-DPI scaling is handled by the toolkit but not yet verified on a real PC.
