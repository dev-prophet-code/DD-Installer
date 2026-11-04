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

A Go core compiled once per OS (no CGO, trivial cross-compilation) with an embedded web GUI served on `127.0.0.1`. Alternatives considered: Electron (large), Tauri/Wails (needs per-OS native toolchains and WebView dependencies; a good later wrapper around the same core - see roadmap).
