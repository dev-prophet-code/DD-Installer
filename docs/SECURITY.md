# Security model

* **No credential storage.** Secrets exist in process memory for the session. The installer writes no config, history or log files.
* **No network listener.** The UI is a native window; there is no local web server, token, or browser involved, so no web attack surface (DNS rebinding, CSRF, XSS) exists.
* **SSH:** host key must be confirmed by the user (SHA256 fingerprint, trust on first use) and is pinned for the run; a changed key aborts. Key passphrases and passwords are never logged.
* **Shell injection:** all user-controlled values are validated against whitelists (names, paths, titles, regions); paths are used inside double quotes after rejecting `` ` $ \ " ' ; & | < > ``; JSON/secret payloads travel in quoted heredocs with random delimiters.
* **Redaction:** exact secret values, token-like `key=value` pairs and the generated admin password printed by upstream are masked in every log line and before the assistant sees output.
* **Assistant:** no destructive actions; moderate fixes need explicit consent; temporary sudo grants are validated with `visudo -c`, removed right after the installation was verified (and the removal is checked), at the latest after 3 hours or at the next boot.
* **Supply chain:** every Dune Docker release is verified with a SHA-256 over all its files before anything is run (see "0.0.5" below); nothing from the internet is piped into a shell.

Report vulnerabilities privately via GitHub security advisories.

## Review & penetration-test log (0.0.1 - 0.0.2 alpha)

Self-conducted review and active tests; fixed before release of the draft:

| Finding | Fix |
|---|---|
| Child processes (apt/sudo/docker) could swallow the rest of a script via shared stdin | All runners use `sh -c 'eval "$(cat)"'` (script fully read first); regression test |
| Shell injection via token / names / title / env values | JSON in quoted random-delimiter heredocs, strict whitelists, `sh` execution test with hostile values |
| UI could lose `done`/`error` events under heavy log output | Index-based event stream; structural events never dropped |
| Assistant aborted on the first manual rule, ignoring fixable matches; "wait 5 min" rules ran only once | Manual rules no longer short-circuit; transient rules allow up to 3 uses |
| `$HOME` in the admin-password path was not expanded by the helper | `os.path.expandvars` |
| Re-running on an installed host was blocked by "port busy" | Downgraded to a warning |
| Over-broad `tls` pattern could misfire | Tightened |
| Runner leaked after SSH reconnect | `install.Run` owns and closes the runner |

The temporary web GUI of 0.0.1-alpha was removed in 0.0.2-alpha (native UI); its checks (token, Host/Origin, CSRF, CSP, traversal) are obsolete because nothing listens on the network any more. The disclaimer gate is enforced in `app.Controller.Start` and tested in `internal/ui`.

Known limitations at that time: Windows `wsl.exe --exec` quoting was untested on real hardware.

## SSH key helper

The only credential the installer can write to disk, and only on an explicit click: an Ed25519 key pair in `~/.ssh` (private key mode 0600, directory 0700). Existing files are never overwritten (exclusive create), names are validated against path traversal, an optional passphrase encrypts the key. A half-created pair is removed on error. The passphrase is kept only in memory.

## WSL2 provisioning (Windows)

The user created in a fresh Ubuntu gets passwordless sudo (the installer runs unattended inside a dedicated distribution). Elevation uses the standard Windows UAC prompt; the installer itself never runs elevated. The consent dialog states what will be installed.

## Reliability and destructive actions (0.0.4)

* **Detached scripts** live in `~/.dd-installer/<step>/` (mode 0700) on the target and contain no secrets; the token and passwords still travel only through the SSH channel in a 0600 temp directory that is removed on exit.
* **known_hosts and jump hosts:** a conflicting `known_hosts` entry is always an error and wins over a session pin; a jump host must already be trusted in `known_hosts`.
* **Firewall helper:** only adds allow rules to an *active* ufw/firewalld; SSH is allowed first; it never enables, disables or resets anything (tested). The web console can be restricted to an IP/CIDR.
* **Management actions:** destructive actions need an exact typed phrase; "remove everything" has hard guard rails (marker files, never `/`, home or system folders) and is tested against real temporary directories.
* **Temporary sudo** is revoked at the end of the run and by a 3-hour timer (systemd-run or a background sleep) in case DD-Installer dies (improved in 0.0.5, see below).
* **Supply chain:** the Dune Docker release is pinned to the tested tag; GitHub Actions are pinned to commit SHAs and updated by Dependabot; releases carry a build provenance attestation and `SHA256SUMS`; CI runs govulncheck and the race detector.
* **Fuzzing:** parsers of untrusted text (scan output, tokens, ssh config, poll results, jump hosts, progress lines, validators) have fuzz tests; fuzzing already found and fixed a quadratic line splitter, a polling race and an ssh-config `Match` bug.

## Review and penetration test (0.0.5)

An internal code review and penetration test (shell injection through every input, host-key policy against a real in-process SSH server, guard rails of destructive actions, proxy behaviour, fuzzing) led to these fixes. Each one has a regression test.

| Finding | Fix |
|---|---|
| The downloaded release was not verified (empty checksum); "newest" piped upstream's `bootstrap.sh` into a shell | SHA-256 over all files, read from `upstream/upstream.json` before every download (offline fallback built in); no checksum, a mismatch or links in the archive stop the install; no piping into a shell |
| Passwords and the Funcom token could be sent to an HTTP proxy from the environment by the helper on the server | The helper checks for proxy settings and connects to the local console directly; health checks use `curl --noproxy '*'` |
| Temporary sudo stayed after a reboot (the timer was transient) | Final check of the installation, then immediate removal and a check that sudo is really gone; a systemd unit removes the grant at the next boot |
| A key marked `@revoked` could be accepted after confirming its fingerprint | Revoked keys are always refused and cannot be confirmed |
| A wrong password on a trusted host looked like a host-key question | Only real host-key decisions are reported as such |
| SSH user and host were only validated in the UI; the user went into the sudoers file | `Config.Validate` applies the same strict checks; the sudoers line is quoted and the user re-validated |
| The uninstall guard could be bypassed with `~//` (home wiped if it looked like a Dune folder) | The path is resolved first; home and every folder containing it are always refused |
| The unencrypted web console was only mentioned in the log | A notice in the GUI that must be acknowledged before installing |
| The install log (with the generated admin password) stayed on the server | Private folder (umask 077), deleted when the step has finished |

What held up: 115 special characters accepted in the install folder were all kept literal in a real shell; tokens, names and titles travel as JSON in quoted heredocs; the tuning whitelist; exclusive key creation; conflicting `known_hosts` entries; fuzzers for all parsers; race tests.

Open points: the web console speaks plain HTTP (a limit of Dune Docker); on systems without systemd the boot-time removal of the temporary sudo grant is not available (the installer still removes it right after the installation, and a background timer covers a crash); the release files are not code-signed yet.

Report vulnerabilities as described in the root [SECURITY.md](../SECURITY.md).
