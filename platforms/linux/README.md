# Linux

* Code: `linux.go` (build tag `linux`). Local install runs the upstream installer natively via `sh`.
* Requirements: regular user with sudo (the installer refuses root), glibc/musl independent static binary.
* Build: `GOOS=linux GOARCH=amd64 go build ./cmd/dd-installer` (also `arm64` for use as an SSH client).
* Packaging ideas: tar.gz (built by `scripts/build.sh`), AppImage/.deb later.
* Browser opening uses `xdg-open`/`gio`; over SSH/headless use `--no-browser` and port-forward the printed port.
