# Sizing & tuning

## Hard requirements (block installation)

x86-64 Linux, AVX2, >= 20 GB RAM, regular (non-root) user.

## Warnings

Layout needs more RAM than available; < 200 GB free; < 4 CPU threads; no passwordless sudo; WSL2 without systemd; ports 31982/31983/15432 or the web console port already in use (expected when Dune Docker is already installed, so a re-run is possible); missing curl/tar (installed automatically).

## RAM estimate (heuristic)

| Component | GB |
|---|---|
| Base (core services + primary Hagga + social maps) | 20 |
| Each extra Sietch | +10 |
| Each Deep Desert instance | +10 |

Anchored on the upstream table (20 basic / 30 extra maps / 40 with Deep Desert / 60+ heavy) and "one Survival server idles at 10-12 GB". **This is an estimate**; calibrate with `dune memory status` / the Console's live memory view and adjust the constants in `internal/sizing`.

## Recommended tuning (applied if enabled)

| Setting | Value | Why |
|---|---|---|
| `DUNE_PG_SHM_SIZE` | 512m / 768m / 1g by RAM | PostgreSQL shared memory |
| `DUNE_PG_JIT` | `off` | avoids JIT latency spikes |
| `DUNE_PG_RANDOM_PAGE_COST` | `1.1` | SSD/NVMe storage |
| `DUNE_RMQ_ERL_ARGS` | `+S 2:2 +sbwt none ...` (<= 8 threads) | leave cores to game servers |
| `dune memory-swap enable` | if RAM < estimate and no swap | bounded safety pool, not a RAM replacement |
| `.wslconfig` | `memory=<n>GB` | WSL2 caps memory at 50% by default |

Values are written only if they match a strict character whitelist.
