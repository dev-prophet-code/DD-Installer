# Choosing and preparing a server

DD-Installer works with any Linux server you can reach over SSH. This page lists what to look for; it does not recommend a company.

## Requirements to compare

| Item | Minimum | Recommended |
|---|---|---|
| CPU | x86-64 with **AVX2** | 8+ threads, dedicated cores |
| RAM | 20 GB | 30-40 GB (more Sietches / Deep Desert), 60 GB+ for heavy use |
| Disk | 200 GB | SSD or NVMe, 300 GB+ with backups |
| OS | Ubuntu 22.04 / 24.04 LTS | Ubuntu 24.04 LTS, fresh install |
| Network | public IPv4 | generous traffic allowance, UDP not filtered |

Pitfalls:

* **ARM servers** (e.g. Ampere, Graviton, Raspberry Pi) do **not** work: the game server is x86-64 only.
* Some virtual servers hide AVX2. Choose "dedicated CPU" / "general purpose" plans or ask the provider; DD-Installer's scan tells you immediately.
* **Shared / burstable** plans with very low RAM (8-16 GB) will fail the 20 GB check.
* Container-based "VPS" (LXC/OpenVZ without Docker support) cannot run Docker. Use KVM-style virtual servers or dedicated machines.

## First login checklist

1. Add your **public SSH key** in the provider's panel when you create the server (see [SSH-GUIDE.md](SSH-GUIDE.md)).
2. Log in as the provider's default user (often `root`, `ubuntu` or `debian`).
3. If it is `root`, create a normal user with sudo (SSH guide, step 1) and copy your key to it.
4. `sudo apt update && sudo apt -y upgrade`, reboot if asked.
5. Open the **cloud firewall** ports ([ROUTER-AND-FIREWALL.md](ROUTER-AND-FIREWALL.md)).
6. Start DD-Installer, choose *Remote Linux server (SSH)* and follow the wizard.

## Official Funcom Hyper-V server

If you run Funcom's official Hyper-V server on Windows you can keep it and use **Manage existing server... > Install the Director travel fix** against the game-server VM (enable SSH in the VM first). Alternatively move to Dune Docker with the upstream migration guide: <https://docs.dunedocker.app/getting-started/migrate-hyper-v-database>.
