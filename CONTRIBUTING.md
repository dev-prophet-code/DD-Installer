# Contributing

Thanks for helping! This is an early alpha; small, focused changes are easiest to review.

## Setup

* Go 1.24+ and a C compiler (the native GUI uses CGO): Linux `sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev`, macOS Xcode command line tools, Windows MinGW-w64.
* `go test ./...` must pass. `go vet ./...` and `gofmt -l .` must be clean.
* UI tests run headless (Fyne test driver). To look at the real window on Linux without a display use `xvfb-run`.

## Layout

```
cmd/dd-installer     entry point
internal/            model, probe, sizing, assistant, install, manage, transport, sshkey, sshconfig, app (controller), ui (Fyne)
platforms/           linux/ macos/ windows/  OS specific code and packaging
docs/                documentation (English)
scripts/             build, release and maintenance scripts
```

The UI only talks to `internal/app.Controller`. OS-specific code lives in `platforms/<os>` behind `internal/platform.Platform`.

## Rules of thumb

* **Never** put a user-controlled value into a shell script without a strict whitelist (`internal/model`) or `shq`. Add a test with hostile values.
* **Never** log or store secrets. Anything shown in the UI must pass the redactor.
* The assistant must never run destructive commands (a test enforces this). Destructive *user* actions need a typed phrase and guard rails (see `internal/manage`).
* New behaviour needs a test; parsers of untrusted text need a fuzz test.
* English only in the UI and docs for now.
* Keep the disclaimer in one place: edit `internal/app/disclaimer.md`, then run `scripts/sync-disclaimer.sh`.

## Common tasks

| Task | How |
|---|---|
| Add an assistant rule | `internal/assistant/rules.go` (+ case in `assistant_test.go`) |
| Update the tested upstream version | `scripts/update-upstream-pin.sh [tag]`, run tests, commit |
| Regenerate README screenshots | `DD_SHOTS=$PWD/docs/images go test ./internal/ui -run TestShots` |
| Release a new version | change `VERSION`, add a section to `CHANGELOG.md`, push to `main` |

## Pull requests

Describe what changed and why, how you tested it, and which OS you tried it on. Do not include real credentials or server addresses in logs or screenshots.
