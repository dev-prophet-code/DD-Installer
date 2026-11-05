# Troubleshooting DD-Installer

For problems with the game server itself see the [official troubleshooting guide](https://docs.dunedocker.app/operations/troubleshooting).

## The app does not start or looks wrong

| Symptom | Cause and fix |
|---|---|
| Linux: nothing happens / "cannot open display" | The app needs a desktop session (X11 or Wayland) with OpenGL. On a server without a desktop, run DD-Installer on your own computer and choose the SSH target. |
| Windows: SmartScreen "unknown publisher" | Not code-signed yet. *More info > Run anyway*. Verify the download with `SHA256SUMS`. |
| macOS: "cannot be opened because the developer cannot be verified" | Right-click `DD-Installer.app` > *Open*, or `xattr -d com.apple.quarantine DD-Installer.app`. |
| Animations are slow / distracting | Click **Effects: on** (top right) to switch them off, or start with `DD_INSTALLER_NO_EFFECTS=1`. |
| Console windows flash on Windows | Fixed in 0.0.3-alpha. If you still see it, tell us at which step. |

## Connecting

See the table at the end of the [SSH guide](SSH-GUIDE.md). Extras:

* *Jump host* needs the jump host in `~/.ssh/known_hosts`: connect to it once with `ssh` in a terminal.
* "Host key CHANGED": the server was reinstalled **or** someone intercepts the connection. Do not continue until you know why.
* Windows ssh-agent: `Get-Service ssh-agent | Set-Service -StartupType Automatic; Start-Service ssh-agent`, then `ssh-add`.

## During the scan

| Message | Meaning |
|---|---|
| `AVX2` error | The CPU or VM does not expose AVX2. Enable host CPU passthrough or choose another server. |
| RAM below 20 GB | Not enough memory; choose a bigger server. |
| Layout needs more RAM than available | A warning: reduce Sietches / Deep Desert or add RAM. |
| Ports "already in use" | Expected if Dune Docker is already installed there (resume); otherwise another program uses them. |

## During the installation

1. Read the **assistant panel**: it names the problem and what it did or what you must do.
2. Fix the cause (or click *Apply fix*), then press **Install** again - finished steps are skipped.
3. Use the log filter (e.g. `error`) to find the relevant lines. Secrets are masked in the log.

Common cases:

| Problem | What happens / what to do |
|---|---|
| Missing curl/tar | Installed automatically. |
| Package manager busy | The assistant waits and retries (apt locks on fresh servers). |
| Docker permission denied | The assistant offers to add your user to the `docker` group (needs your click). |
| `sudo` asks for a password | Enter the server password in the SSH settings; the assistant can grant temporary passwordless sudo (removed afterwards, at the latest after 3 hours). |
| SteamCMD failures | The assistant runs `dune update fix-steamcmd` and retries after a pause. |
| Connection lost | The app reconnects by itself; the install step keeps running on the server. |
| WSL needs a restart | Restart Windows, start DD-Installer again, continue. |
| Funcom token rejected | Generate a new token in your Funcom account and enter it again. |

## After a failure: how do I start over cleanly?

Usually you do not need to - press **Install** again. If you really want to remove a half-finished server: *Manage existing server... > Remove everything including data* (typed confirmation).

## Reporting a bug

Open an issue (template provided) with: DD-Installer version, your OS, target OS, the step that failed and the **log lines around the error** (the log masks secrets, but check before posting).
