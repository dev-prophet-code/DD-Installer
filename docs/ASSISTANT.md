# Problem-solving assistant

When an install step fails, `internal/assistant` matches the redacted tail of the output and the scanned host facts against rules derived from <https://docs.dunedocker.app/> and the upstream installer's own messages.

| Risk | Behaviour |
|---|---|
| `safe` | Applied automatically (idempotent, scoped to the install) |
| `moderate` | Explained; runs only after you click **Apply fix** (group/sudoers/system changes) |
| `manual` | Instructions only |

The assistant **never** runs destructive actions (no `rm -rf`, volume/prune, disk changes - enforced by a unit test). A fix rule is applied once per run (transient problems such as package locks, outages and rate limits up to 3 times); a step is retried at most 3 times. Unknown errors yield a pointer to the troubleshooting docs and `dune doctor`.

## Rule base

| Rule | Problem | Fix |
|---|---|---|
| missing-tools | curl/tar/openssl/python3/gpg missing | install via apt/dnf/yum/zypper/pacman/apk |
| pkg-lock | dpkg/apt lock held | wait and retry |
| install-dir-exists | folder already holds Dune Docker | reuse, never overwrite |
| port-in-use | web port taken | next free port |
| docker-socket | no Docker permission | add user to `docker`, reconnect (asks first) |
| runtime-permissions | files owned by root | upstream `repair-host-runtime-permissions.sh` |
| steamcmd | game download fails | `dune update fix-steamcmd`, wait, retry |
| network-risk | NAT bind risk | `dune network fix` |
| console-down | Console not answering | restart container, show log, retry |
| login-rate-limit | HTTP 429 | wait, retry |
| github-unreachable | DNS/TLS/GitHub errors | diagnostics, wait, retry |
| sudo-password | sudo needs a password | temporary NOPASSWD grant (asks first, removed at the end) |
| wsl-systemd | systemd off in WSL2 | write `/etc/wsl.conf` (asks first) |
| no-space, avx, root-install, funcom-token, docker-desktop-internal, ssh-* | cannot be fixed automatically | explanation + exact steps |

Add a rule in `internal/assistant/rules.go` with a regex, an explanation, a risk level and actions; add a case to `assistant_test.go`.
