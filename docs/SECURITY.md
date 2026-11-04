# Security model

* **No credential storage.** Secrets exist in process memory for the session. The installer writes no config, history or log files.
* **No network listener.** The UI is a native window; there is no local web server, token, or browser involved, so no web attack surface (DNS rebinding, CSRF, XSS) exists.
* **SSH:** host key must be confirmed by the user (SHA256 fingerprint, trust on first use) and is pinned for the run; a changed key aborts. Key passphrases and passwords are never logged.
* **Shell injection:** all user-controlled values are validated against whitelists (names, paths, titles, regions); paths are used inside double quotes after rejecting `` ` $ \ " ' ; & | < > ``; JSON/secret payloads travel in quoted heredocs with random delimiters.
* **Redaction:** exact secret values, token-like `key=value` pairs and the generated admin password printed by upstream are masked in every log line and before the assistant sees output.
* **Assistant:** no destructive actions; moderate fixes need explicit consent; temporary sudo grants are validated with `visudo -c` and revoked on exit.
* **Supply chain:** the installer runs upstream `bootstrap.sh` from GitHub (HTTPS). Pinning/verification is on the roadmap; review upstream before use.

Report vulnerabilities privately via GitHub security advisories.

## Review & penetration-test log (v0.1 draft)

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

The v0.1 web GUI was removed in v0.2 (native UI); its checks (token, Host/Origin, CSRF, CSP, traversal) are obsolete because nothing listens on the network any more. The disclaimer gate is enforced in `app.Controller.Start` and tested in `internal/ui`.

Known limitations: SSH keyboard-interactive auth is not supported; Windows `wsl.exe --exec` quoting is untested on real hardware; the upstream installer is fetched from `main` (not pinned).

## WSL2 provisioning (Windows)

The user created in a fresh Ubuntu gets passwordless sudo (the installer runs unattended inside a dedicated distribution). Elevation uses the standard Windows UAC prompt; the installer itself never runs elevated. The consent dialog states what will be installed.
