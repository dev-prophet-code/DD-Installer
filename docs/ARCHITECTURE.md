# Architecture

```
cmd/dd-installer          main, platform registration (build tags)
internal/model            Config + validation (pure)
internal/probe            host scan script + parser
internal/sizing           requirements, RAM estimate, tuning recommendations
internal/assistant        rule base + diagnosis (problem solver)
internal/install          step orchestration, scripts, detached runner, firewall, upstream pin, redaction, assistant loop
internal/manage           actions for existing servers (update, backup, stop, remove, Hyper-V fix)
internal/sshkey           SSH key helper
internal/sshconfig        ~/.ssh/config reader
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

Every step runs through `runScript`, which streams redacted output to the UI. On failure the **assistant** diagnoses, optionally fixes and the step is retried (at most 3 attempts per step; a fix rule is applied once per run, transient rules such as package locks or outages up to 3 times).

Running the Console API calls on the target means no extra firewall port has to be reachable from your computer, and secrets travel only through the SSH channel.

## Platform layer

`internal/platform.Platform` provides `LocalSupport`, `LocalRunner`, `ProvisionLocal`, `DefaultSSHKeys`. Each OS package registers itself from a build-tagged file, so a binary contains only its own OS code.

## Native UI

`internal/ui` is a Fyne application (real OS windows and widgets, no browser, no embedded web server). It only talks to `internal/app.Controller`, so another front end (CLI, different toolkit) could reuse the whole core. UI updates from background work go through `fyne.Do`. Headless tests use Fyne's test driver.

## Windows: WSL2 provisioning

`LocalSupport` reports `CanProvision` when `wsl.exe` or an Ubuntu distribution is missing. The wizard then offers **Install WSL2 + Ubuntu**: an elevated (UAC) `wsl --install -d Ubuntu --no-launch`, then, as root in the new distribution, a user `dune` with passwordless sudo, `systemd=true` and `default=dune` in `/etc/wsl.conf`, followed by `wsl --terminate`. If Windows needs a restart (exit code 3010, or the distribution is not registered yet) the user is told to reboot and start the installer again; the setup then continues with the installed Ubuntu.

## Reliability model

* **Detached steps.** The install step (Docker, web console) is started on the target with `setsid nohup`, writes to `~/.dd-installer/<step>/log` and records its exit code. DD-Installer polls the log with short SSH calls (offset based, state read before size so no line is lost). If a poll fails because the connection broke, the runner reconnects with back-off and keeps reading. Cancelling kills the remote process group. The scripts contain no secrets.
* **Idempotent steps and resume.** Every step may have a *check* script that exits 0 when the step is already done (release files present, console healthy, setup complete, battlegroup ready). `Run` skips such steps, so pressing Install again after any failure continues from where it stopped. The console helper re-attaches to a running `init` task instead of starting a second one. Marker lines (`DD_PORT`, `DD_UPSTREAM_VERSION`) carry facts such as the real console port back to later steps.
* **Connection loss** during non-detached steps (console API calls run on the target) is detected (`transport.IsConnectionLost`) and the step is repeated after reconnecting; this is safe because the steps are idempotent.
* **Pinned upstream.** `internal/install/upstream.go` holds the tested tag (and optional SHA-256). The pinned downloader repeats the safety checks of upstream's `bootstrap.sh`. A version mismatch is logged as a warning.
* **Sleep.** `Platform.PreventSleep` (SetThreadExecutionState / caffeinate / systemd-inhibit) while a run or action is active.

## SSH

`transport.DialSSH` order of trust: a matching entry in `~/.ssh/known_hosts` is trusted; a conflicting entry is an error; otherwise the fingerprint the user confirmed in this session must match. One jump host (ProxyJump) is supported and must be in `known_hosts`. Password auth also answers keyboard-interactive prompts. The agent is `SSH_AUTH_SOCK` or, on Windows, the OpenSSH named pipe.

## Management actions

`internal/manage` defines actions as scripts with a risk level. `app.Controller.RunAction` refuses to run while an installation is active and requires the exact typed phrase for destructive actions. The "remove everything" script refuses unless the folder contains `install.sh`, `docker-compose.yml` and `runtime/`, and never touches `/`, the home directory or system folders (tested against real directories).
