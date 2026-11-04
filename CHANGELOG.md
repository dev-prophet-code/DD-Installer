# Changelog

Versions follow the number in the [`VERSION`](VERSION) file. Newest first.

## 0.0.2-alpha

* Native desktop application (Fyne) for Windows, macOS and Linux; the web interface was removed.
* Windows: detects WSL2/Ubuntu, offers to install them (UAC), prepares a user with systemd and continues the setup.
* Versioned releases built automatically per operating system (CI).
* Disclaimer shown in the app and required before installing.
* Review and self-conducted penetration test fixes (stdin-safe script execution, shell-injection tests, assistant retry logic, event delivery).

## 0.0.1-alpha

* First draft: Go core (model, scan, sizing, problem-solving assistant, SSH/local transport), temporary local web UI, documentation, per-OS folders.
