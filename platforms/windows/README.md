# Windows

* Code: `windows.go` (build tag `windows`).
* **Local install** runs inside an **Ubuntu WSL2 distribution** via `wsl.exe -d <distro> -- sh -s`. Docker Desktop's internal `docker-desktop` distributions are never used (upstream forbids it).
* Prerequisites: `wsl --install -d Ubuntu`, create your Linux user, enable systemd in `/etc/wsl.conf`, Docker Desktop with WSL integration for that distro (or a native Docker engine in WSL).
* WSL2 limits memory (default 50%). The scan suggests a `%UserProfile%\.wslconfig`; run `wsl --shutdown` afterwards.
* **Remote install** uses the built-in Go SSH client - no OpenSSH or PuTTY required.
* Build: `GOOS=windows GOARCH=amd64 go build -o dd-installer.exe ./cmd/dd-installer`. Signed MSI is on the roadmap (unsigned exe triggers SmartScreen).
