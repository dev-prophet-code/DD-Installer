# Roadmap

Done in 0.0.4: detached/resumable install, pinned upstream, firewall helper, management actions, Hyper-V fix, finish page, SSH config/known_hosts/jump host/Windows agent, effects switch, controller and SSH tests, fuzzing, release automation.

## Next

* **End-to-end validation** on real servers (Ubuntu 22.04/24.04 over SSH, local Windows 11 + WSL2) and on real Windows/macOS machines (HiDPI, WSL provisioning, `.icns`, exe icon). A Docker-based sshd integration test in CI.
* **Calibrate the RAM heuristic** with real `dune memory status` data; "use recommended layout" button.
* **Verify per-Sietch PvP/PvE** in game and drop the "experimental" label; show the existing Deep Desert role.
* **SHA-256 for the pinned release** (run `scripts/update-upstream-pin.sh` on a machine with GitHub access and commit).
* **Signing:** Windows certificate, Apple Developer ID + notarisation; Linux arm64 build.
* Copy-log/save-log and "report a problem" with a pre-filled issue; accessibility (keyboard order, font size, contrast); window size memory; a German translation.
* Pageant support, more than one jump host, adding trusted keys to `known_hosts`.
* Optional: native per-OS UIs on top of the same controller.
