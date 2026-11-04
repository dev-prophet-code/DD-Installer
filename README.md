# DD-Installer

A cross-platform **GUI installer for [Dune: Awakening Docker](https://github.com/Red-Blink/dune-awakening-selfhost-docker)** (self-hosted Dune: Awakening dedicated server). One binary per operating system - **Linux, macOS, Windows** - installs the server either on **this computer** or on a **remote Linux server over SSH**.

> **Status: first draft (v0.1).** The core logic is implemented and unit-tested; end-to-end runs against real servers still need validation. See [docs/ROADMAP.md](docs/ROADMAP.md).

> **Disclaimer.** Unofficial community tool, not affiliated with Funcom or RedBlink. DD-Installer **does not store your credentials** (SSH password, key passphrase, Funcom token are kept in memory only), and **neither RedBlink (author of Dune: Awakening Docker) nor Prophet (author of DD-Installer) can be held liable** for any damage. Full text: [docs/DISCLAIMER.md](docs/DISCLAIMER.md).

## Features

| | |
|---|---|
| **Two targets** | Local install (Linux native, Windows via WSL2) or remote install over SSH (key, password or ssh-agent, trust-on-first-use host-key check) |
| **Sietches** | Choose how many Hagga Basin Sietches to create initially, name each one, and mark each PvE or PvP (PvP/PvE per Sietch is *experimental*) |
| **Deep Desert** | 1-3 instances; ruleset handling follows upstream rules (see below) |
| **System scan** | Probes CPU (AVX2), RAM, swap, disk, Docker, sudo, ports, WSL; blocks impossible setups; recommends a layout and tuning (PostgreSQL, RabbitMQ, swap, `.wslconfig`) |
| **Problem-solving assistant** | When a step fails, the failure is matched against a rule base built from the official docs; safe fixes are applied automatically, risky ones need your approval, everything is explained ([docs/ASSISTANT.md](docs/ASSISTANT.md)) |
| **Secret hygiene** | Nothing is written to disk by the installer; secrets are redacted from logs and from what the assistant sees |

## How it works

DD-Installer does not re-implement the server. It drives the **upstream installer** (`bootstrap.sh` / `install.sh`) and the **Dune Docker Console API** on the target, then applies your layout with the upstream CLI/API (`sietches set-active`, `set-display`, `deepdesert layout set`). Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

```
GUI (browser/app window, 127.0.0.1 only)
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

**[Releases](../../releases)** - `latest` is the rolling build of `main`; tagged versions (`v0.x.y`) are regular releases.

| OS | File |
|---|---|
| Windows (x64) | `dd-installer-windows-amd64.zip` |
| macOS (Apple Silicon / Intel) | `dd-installer-macos-arm64.tar.gz` / `...-amd64.tar.gz` |
| Linux (x64 / arm64) | `dd-installer-linux-amd64.tar.gz` / `...-arm64.tar.gz` |

Verify with `SHA256SUMS`. The binaries are **not code-signed** yet: Windows SmartScreen and macOS Gatekeeper will warn (see the platform folders). To publish a version: `git tag v0.1.0 && git push origin v0.1.0`.

## Build & run

Requires Go 1.24+. No CGO, so cross-compiling needs no extra toolchains.

```bash
go test ./...
./scripts/build.sh            # builds dist/ for all platforms
go run ./cmd/dd-installer     # opens the GUI in your browser
```

Flags: `--no-browser` (print the URL only), `--version`.

## Repository layout

```
cmd/dd-installer/    entry point (+ build-tagged platform imports)
internal/            model, probe, sizing, assistant, install, transport, gui (shared core)
platforms/           linux/  macos/  windows/   per-OS code, packaging notes, CI notes
docs/                architecture, feasibility, sizing, assistant, security, disclaimer, roadmap
scripts/             build helpers
```

## Documentation

[Feasibility study](docs/FEASIBILITY.md) - [Architecture](docs/ARCHITECTURE.md) - [Sizing](docs/SIZING.md) - [Assistant](docs/ASSISTANT.md) - [Security](docs/SECURITY.md) - [Disclaimer](docs/DISCLAIMER.md) - [Roadmap](docs/ROADMAP.md)

Upstream documentation: <https://docs.dunedocker.app/>
