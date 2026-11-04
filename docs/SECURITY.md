# Security model

* **No credential storage.** Secrets exist in process memory for the session. The installer writes no config, history or log files.
* **GUI server:** binds `127.0.0.1` on a random port; per-launch 192-bit token (URL exchanged for an HttpOnly, SameSite=Strict cookie); `Host` and `Origin` validation (DNS-rebinding / CSRF); JSON-only POSTs; 1 MiB body limit; unknown JSON fields rejected; strict CSP, no inline script, UI built with `textContent` only.
* **SSH:** host key must be confirmed by the user (SHA256 fingerprint, trust on first use) and is pinned for the run; a changed key aborts. Key passphrases and passwords are never logged.
* **Shell injection:** all user-controlled values are validated against whitelists (names, paths, titles, regions); paths are used inside double quotes after rejecting `` ` $ \ " ' ; & | < > ``; JSON/secret payloads travel in quoted heredocs with random delimiters.
* **Redaction:** exact secret values, token-like `key=value` pairs and the generated admin password printed by upstream are masked in every log line and before the assistant sees output.
* **Assistant:** no destructive actions; moderate fixes need explicit consent; temporary sudo grants are validated with `visudo -c` and revoked on exit.
* **Supply chain:** the installer runs upstream `bootstrap.sh` from GitHub (HTTPS). Pinning/verification is on the roadmap; review upstream before use.

Report vulnerabilities privately via GitHub security advisories.
