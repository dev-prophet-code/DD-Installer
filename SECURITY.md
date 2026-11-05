# Security policy

DD-Installer installs software and handles credentials (SSH access, a Funcom token), so security reports are taken seriously.

## Supported versions

Only the latest release (see [Releases](../../releases)) receives fixes. This is an early alpha.

## Reporting a vulnerability

Please **do not open a public issue**. Use GitHub's private reporting: **Security > Report a vulnerability** on this repository. Include the version, your OS, what you observed and, if possible, steps to reproduce. Do not include real passwords, tokens or private keys.

You can expect an acknowledgement within a few days. Fixes are published in a new release and mentioned in the changelog.

## What counts

Examples we want to hear about: commands injected through user input, credentials leaking into logs or files, a way to make the installer run destructive commands without consent, bypassing the SSH host-key checks, unsafe handling of downloaded files.

Out of scope: problems in Dune Docker itself (report them to its authors), in Funcom's software, or social engineering.

## How the project protects users

See [docs/SECURITY.md](docs/SECURITY.md): no credential storage, strict input whitelists, SSH host-key policy, redacted logs, guard rails for destructive actions, pinned upstream and pinned build actions, provenance attestation of release files.
