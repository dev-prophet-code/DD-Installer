# DD-Installer

A cross-platform **native desktop installer for [Dune: Awakening Docker](https://github.com/Red-Blink/dune-awakening-selfhost-docker)** (self-hosted Dune: Awakening dedicated server). One real application per operating system - **Linux, macOS, Windows** (native window, no web interface, no browser) - installs the server either on **this computer** or on a **remote Linux server over SSH**.

> **Status: alpha (v0.0.2-alpha).** The core logic is implemented and unit-tested; end-to-end runs against real servers still need validation. See [docs/ROADMAP.md](docs/ROADMAP.md).

> **Disclaimer.** Unofficial community tool, not affiliated with Funcom or RedBlink. DD-Installer **does not store your credentials** (SSH password, key passphrase, Funcom token are kept in memory only), and **neither RedBlink (author of Dune: Awakening Docker) nor Prophet (author of DD-Installer) can be held liable** for any damage. Full text: [docs/DISCLAIMER.md](docs/DISCLAIMER.md).

## Features

| | |
|---|---|
| **Two targets** | Local install (Linux native, Windows via WSL2 - **the installer checks for WSL2/Ubuntu, offers to install it and then continues**) or remote install over SSH (key, password or ssh-agent, trust-on-first-use host-key check) |
| **Sietches** | Choose how many Hagga Basin Sietches to create initially, name each one, and mark each PvE or PvP (PvP/PvE per Sietch is *experimental*) |
| **Deep Desert** | 1-3 instances; ruleset handling follows upstream rules (see below) |
| **System scan** | Probes CPU (AVX2), RAM, swap, disk, Docker, sudo, ports, WSL; blocks impossible setups; recommends a layout and tuning (PostgreSQL, RabbitMQ, swap, `.wslconfig`) |
| **Problem-solving assistant** | When a step fails, the failure is matched against a rule base built from the official docs; safe fixes are applied automatically, risky ones need your approval, everything is explained ([docs/ASSISTANT.md](docs/ASSISTANT.md)) |
| **Secret hygiene** | Nothing is written to disk by the installer; secrets are redacted from logs and from what the assistant sees |

## How it works

DD-Installer does not re-implement the server. It drives the **upstream installer** (`bootstrap.sh` / `install.sh`) and the **Dune Docker Console API** on the target, then applies your layout with the upstream CLI/API (`sietches set-active`, `set-display`, `deepdesert layout set`). Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

```
Native window (Fyne) - no web server, no network listener
  -> Go core: validate -> scan -> sizing -> install steps -> assistant
       -> transport: local shell | WSL2 | SSH
            -> target: bootstrap.sh -> install.sh -> Console API (127.0.0.1 on the target)
```

## Platform support

| OS | Install on this computer | Install on a remote server (SSH) | Folder |
|---|---|---|---|
| Linux | yes (native; regular user with sudo) | yes | [platforms/linux](platforms/linux) |
| Windows | yes, **via Ubuntu WSL2** | yes | [platforms/windows](platforms/windows) |
| macOS | **no** (x86-64 + AVX2 needed, not available) - see [docs/FEASIBILITY.md](docs/FEASIBILITY.md) | yes | [platforms/macos](platforms/macos) |

The *target* must be x86-64 Linux with AVX2, >= 20 GB RAM (30-40 GB+ with more maps), >= 200 GB disk and a regular user with sudo (Ubuntu recommended).

## Important rules inherited from upstream

* **Deep Desert rulesets:** upstream keeps the first instance's existing PvE/PvP role, gives the second the *opposite*, and only the third is freely selectable.
* **Sietch names:** 1-32 letters, digits, space, `_`, `-` (conservative; upstream/Funcom may reject others).
* **Never install as root.** The installer refuses it.

## Download

Ready-made programs are built automatically by GitHub Actions on every push to `main`:

**[Releases](../../releases)** - versioned (`v0.0.2-alpha`, ...). The version number lives in the [`VERSION`](VERSION) file; versions with `-alpha`/`-beta` are marked as pre-releases.

| OS | File |
|---|---|
| Windows (x64) | `dd-installer-windows-amd64.zip` |
| macOS (Apple Silicon / Intel) | `dd-installer-macos-arm64.tar.gz` / `...-amd64.tar.gz` |
| Linux (x64) | `dd-installer-linux-amd64.tar.gz` |

Verify with `SHA256SUMS`. The binaries are **not code-signed** yet: Windows SmartScreen and macOS Gatekeeper will warn (see the platform folders). To publish a new version: change the number in `VERSION` and push to `main` - the release `v<VERSION>` is built and published automatically (pushing the same version again replaces that release).

## Build & run

Requires Go 1.24+ and a C compiler (the native GUI uses CGO): Linux `gcc libgl1-mesa-dev xorg-dev`, macOS Xcode command line tools, Windows MinGW-w64. Each OS is built on its own machine (CI does this on GitHub runners).

```bash
go test ./...
./scripts/build.sh            # builds + packages for the OS you are on into dist/
go run ./cmd/dd-installer     # opens the installer window
```

Flag: `--version`. A desktop session is required (no headless mode).

## Repository layout

```
cmd/dd-installer/    entry point (+ build-tagged platform imports)
internal/            model, probe, sizing, assistant, install, transport, app (controller), ui (native Fyne UI)
platforms/           linux/  macos/  windows/   per-OS code, packaging notes, CI notes
docs/                architecture, feasibility, sizing, assistant, security, disclaimer, roadmap
scripts/             build helpers
```

## Documentation

[Feasibility study](docs/FEASIBILITY.md) - [Architecture](docs/ARCHITECTURE.md) - [Sizing](docs/SIZING.md) - [Assistant](docs/ASSISTANT.md) - [Security](docs/SECURITY.md) - [Disclaimer](docs/DISCLAIMER.md) - [Roadmap](docs/ROADMAP.md)

Upstream documentation: <https://docs.dunedocker.app/>
