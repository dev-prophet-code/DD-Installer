# DD-Installer

Native desktop installer for **[Dune: Awakening Docker](https://github.com/Red-Blink/dune-awakening-selfhost-docker)** (self-hosted Dune: Awakening dedicated server).

One real application per operating system - **Windows, macOS, Linux** - with a native window (no web interface, no browser). It installs the server either on **this computer** or on a **remote Linux server over SSH**, creates your Sietches and Deep Desert layout, and tunes the setup to the machine.

> **Status: alpha (`0.0.2-alpha`).** The core logic is implemented and unit-tested. It has **not yet been tested end to end** on real servers or on real Windows/macOS machines. Expect rough edges. See the [roadmap](docs/ROADMAP.md) and the [changelog](CHANGELOG.md).

> **Disclaimer.** Unofficial community tool, not affiliated with Funcom or RedBlink. DD-Installer **does not store your credentials** (SSH password, key passphrase and Funcom token stay in memory for the session only). **Neither RedBlink (author of Dune: Awakening Docker) nor Prophet (author of DD-Installer) can be held liable** for any damage. Full text: [docs/DISCLAIMER.md](docs/DISCLAIMER.md). The app asks you to accept it before installing.

## Download

Get the latest release from the **[Releases page](../../releases)**:

| OS | File |
|---|---|
| Windows (x64) | `dd-installer-windows-amd64.zip` |
| macOS (Apple Silicon) | `dd-installer-macos-arm64.tar.gz` |
| macOS (Intel) | `dd-installer-macos-amd64.tar.gz` |
| Linux (x64) | `dd-installer-linux-amd64.tar.gz` |

Verify the download with `SHA256SUMS`. The programs are **not code-signed yet**, so Windows SmartScreen and macOS Gatekeeper will warn (see the notes in [platforms/](platforms)). Releases are built automatically by GitHub Actions; see [Releases and versions](#releases-and-versions).

## What it does

| Feature | Details |
|---|---|
| **Two targets** | *This computer*: Linux natively, Windows via WSL2. *Remote server*: SSH with key, password or ssh-agent and a confirmed host-key fingerprint. macOS can only use the remote target. |
| **WSL2 on Windows** | Checks for WSL2 and Ubuntu. If missing, it offers to install them (administrator prompt), prepares a user, enables systemd and continues the setup. A Windows restart may be required. |
| **Sietches** | Choose how many Hagga Basin Sietches to create, name each one and set it to PvE or PvP (per-Sietch ruleset is *experimental*). |
| **Deep Desert** | 1-3 instances. Upstream rules apply: the first instance keeps its existing ruleset, the second gets the opposite, only the third is selectable. |
| **System scan and tuning** | Probes CPU (AVX2), RAM, swap, disk, Docker, sudo, ports and WSL; blocks impossible setups; recommends a layout and tuning (PostgreSQL, RabbitMQ, swap, `.wslconfig`). |
| **Problem-solving assistant** | If a step fails, the error is matched against rules built from the official documentation. Safe fixes run automatically, risky ones need your approval, nothing destructive is ever done. See [docs/ASSISTANT.md](docs/ASSISTANT.md). |
| **Secret hygiene** | Nothing is written to disk by the installer; secrets are masked in logs and before the assistant sees any output. |

## Requirements

* **Target server:** x86-64 Linux with AVX2, at least 20 GB RAM (30-40 GB or more with extra Sietches or Deep Desert), 200 GB or more disk, a regular user with sudo (Ubuntu recommended). The installer refuses to run as root.
* **Your computer:** a desktop session on Windows 10/11, macOS or Linux (X11/Wayland with OpenGL).
* **A Funcom self-host service token** for the server.

| OS | Install on this computer | Install on a remote server | Notes |
|---|---|---|---|
| Linux | yes | yes | [platforms/linux](platforms/linux) |
| Windows | yes, via Ubuntu WSL2 | yes | [platforms/windows](platforms/windows) |
| macOS | no (needs x86-64 Linux with AVX2) | yes | [platforms/macos](platforms/macos) |

## How it works

DD-Installer does not re-implement the server. It drives the **upstream installer** (`bootstrap.sh` / `install.sh`) and the **Dune Docker Console API** on the target, then applies your layout with the upstream commands (`sietches set-active`, `set-display`, `deepdesert layout set`).

```
Native window (Fyne) - no web server, no network listener
  -> Go core: validate -> scan -> sizing -> install steps -> assistant
       -> transport: local shell | WSL2 | SSH
            -> target: bootstrap.sh -> install.sh -> Console API (on the target only)
```

More: [Architecture](docs/ARCHITECTURE.md), [Feasibility study](docs/FEASIBILITY.md).

## Build from source

Requires Go 1.24+ and a C compiler because the native GUI uses CGO:

* Linux: `sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev`
* macOS: Xcode command line tools
* Windows: MinGW-w64 (`gcc` on PATH)

```bash
go test ./...
./scripts/build.sh          # builds and packages for the OS you are on into dist/
go run ./cmd/dd-installer   # opens the installer window
```

`dd-installer --version` prints the version. Each OS must be built on its own machine (no cross-compilation); CI does this on GitHub runners.

## Releases and versions

The version number lives in the [`VERSION`](VERSION) file. Every push to `main` builds all three operating systems and publishes the release `v<VERSION>` (versions with `-alpha`/`-beta` are marked as pre-releases). To publish a new version, change `VERSION`, add a [changelog](CHANGELOG.md) entry and push. Pushing the same version again replaces that release.

## Repository layout

```
cmd/dd-installer/   entry point and build-tagged platform registration
internal/           shared core: model, probe, sizing, assistant, install, transport, app (controller), ui (native Fyne UI)
platforms/          linux/  macos/  windows/  per-OS code and notes
docs/               architecture, feasibility, sizing, assistant, security, disclaimer, roadmap
scripts/            build helper
.github/workflows/  CI and release automation
```

## Documentation

[Feasibility](docs/FEASIBILITY.md) | [Architecture](docs/ARCHITECTURE.md) | [Sizing](docs/SIZING.md) | [Assistant](docs/ASSISTANT.md) | [Security](docs/SECURITY.md) | [Disclaimer](docs/DISCLAIMER.md) | [Roadmap](docs/ROADMAP.md) | [Changelog](CHANGELOG.md)

Upstream project and documentation: <https://github.com/Red-Blink/dune-awakening-selfhost-docker>, <https://docs.dunedocker.app/>

## License

No license has been chosen yet, so all rights are reserved by the author until one is added.
