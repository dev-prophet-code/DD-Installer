# Feasibility study

**Question:** can one installer run on macOS, Windows and Linux, install Dune Docker locally or on a remote server over SSH, create a chosen number of named PvE/PvP Sietches and Deep Desert instances, and tune the setup to the host?

**Answer: yes, with three constraints.** Based on the three Red-Blink repositories and <https://docs.dunedocker.app/> (source: `dune-docker-docs`).

## What upstream gives us

| Need | Upstream mechanism |
|---|---|
| Install | `bootstrap.sh` (POSIX sh, downloads latest release, supports `DUNE_INSTALL_DIR`, `DUNE_BOOTSTRAP_SKIP_INSTALL`, `DUNE_RELEASE_SHA256`) then `install.sh` (installs Docker/tools, honours `ADMIN_BIND_PORT`, works without a TTY) |
| First-run config | Console API: `POST /api/setup/write-config`, `/save-token`, `/init`, polled via `/api/setup/tasks/{id}` (session cookie + CSRF header) |
| Sietches | `dune sietches set-max/set-active/set-display`, API `POST /api/sietches/update`, `GET /api/sietches/dimensions?ids=1` |
| Deep Desert | `dune deepdesert layout set <1\|2\|3> --third-role <pve\|pvp>`, API `POST /api/deepdesert/update` |
| Tuning knobs | `.env`: `DUNE_PG_SHM_SIZE`, `DUNE_PG_JIT`, `DUNE_PG_RANDOM_PAGE_COST`, `DUNE_RMQ_ERL_ARGS`; `dune memory-swap`, memory balancer |
| Requirements | AVX/AVX2, 20 GB RAM minimum (30/40/60 GB tiers), 200 GB disk, non-root sudo user |

The Hyper-V repository (`dune-awakening-hyperv-director-travel-fix`) is a separate patch for Funcom's *official* Hyper-V server; it is not part of a Docker install and is out of scope (see roadmap).

## Constraints

1. **The server only runs on x86-64 Linux with AVX2.** Therefore "install on this computer" works natively on Linux, via **Ubuntu WSL2 on Windows**, and **not on macOS** (no AVX2 for x86 Linux containers on Apple hardware). macOS can still be the *client* for SSH installs.
2. **Deep Desert PvE/PvP is constrained by upstream:** the first instance keeps its role, the second gets the opposite, only the third is selectable. The GUI shows this instead of promising free choice.
3. **PvE/PvP per Sietch is not an officially documented one-click feature.** Upstream exposes per-partition selectors (`partition_pvp_enabled` / `partition_pve_enabled`) through the Console settings API. DD-Installer can set them but flags it **experimental**; verify in game.

## Architecture choice

A Go core shared by all platforms plus a **native desktop UI built with Fyne** (real OS windows and widgets - no browser, no web server). Because the GUI toolkit needs CGO, each OS is built on its own machine (GitHub runners). Alternatives considered: a local web UI (rejected: the requirement is a native application), Electron (large, web-based), Wails/Tauri (WebView-based), and three separate native UIs (WinUI, SwiftUI, GTK: best look, triple the code). Because the UI only talks to `internal/app.Controller`, a dedicated per-OS UI can be added later without touching the core.
