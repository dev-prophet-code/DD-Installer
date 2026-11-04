# Architecture

```
cmd/dd-installer          main, platform registration (build tags)
internal/model            Config + validation (pure)
internal/probe            host scan script + parser
internal/sizing           requirements, RAM estimate, tuning recommendations
internal/assistant        rule base + diagnosis (problem solver)
internal/install          step orchestration, scripts, redaction, assistant loop
internal/transport        Runner: Local shell, SSH (host-key pinning)
internal/platform         interface implemented per OS
internal/app              UI-independent controller (scan, install, events, confirmations)
internal/ui               native desktop UI (Fyne): wizard window
platforms/{linux,macos,windows}   OS-specific implementations
```

## Install pipeline

1. **connect** (local runner / WSL2 / SSH with confirmed host key)
2. **scan** - one read-only POSIX script, parsed into `probe.Info`; `sizing.Analyse` produces blocking errors, warnings and tuning
3. **bootstrap** - upstream `bootstrap.sh` with `DUNE_BOOTSTRAP_SKIP_INSTALL=1` (existing complete installs are reused, never overwritten)
4. **install** - upstream `install.sh` with `ADMIN_BIND_PORT`
5. **tuning** - validated `.env` overrides from the sizing report
6. **console-setup** - on the target, an embedded Python (stdlib) helper talks to `http://127.0.0.1:<port>`: login with the generated admin password, write server title/region, save the Funcom token, run the deployment task
7. **start / ready** - start the battlegroup, wait for `dune ready`
8. **layout** - Sietch count, names (and optional PvE/PvP), Deep Desert layout
9. **post** - e.g. enable memory-swap

Every step runs through `runScript`, which streams redacted output to the GUI. On failure the **assistant** diagnoses, optionally fixes and the step is retried (max 3 attempts; a rule is applied at most once per run).

Running the Console API calls on the target means no extra firewall port has to be reachable from your computer, and secrets travel only through the SSH channel.

## Platform layer

`internal/platform.Platform` provides `LocalSupport`, `LocalRunner`, `ProvisionLocal`, `DefaultSSHKeys`. Each OS package registers itself from a build-tagged file, so a binary contains only its own OS code.

## Native UI

`internal/ui` is a Fyne application (real OS windows and widgets, no browser, no embedded web server). It only talks to `internal/app.Controller`, so another front end (CLI, different toolkit) could reuse the whole core. UI updates from background work go through `fyne.Do`. Headless tests use Fyne's test driver.

## Windows: WSL2 provisioning

`LocalSupport` reports `CanProvision` when `wsl.exe` or an Ubuntu distribution is missing. The wizard then offers **Install WSL2 + Ubuntu**: an elevated (UAC) `wsl --install -d Ubuntu --no-launch`, then, as root in the new distribution, a user `dune` with passwordless sudo, `systemd=true` and `default=dune` in `/etc/wsl.conf`, followed by `wsl --terminate`. If Windows needs a restart (exit code 3010, or the distribution is not registered yet) the user is told to reboot and start the installer again; the setup then continues with the installed Ubuntu.
