# macOS

* Code: `macos.go` (build tag `darwin`).
* **Local install is not supported**: Dune Docker needs x86-64 Linux with AVX2, which macOS/Apple Silicon cannot provide. Use the **SSH target** to install on a Linux server.
* Build: `GOOS=darwin GOARCH=arm64|amd64 go build ./cmd/dd-installer`.
* Distribution: unsigned binary for now; notarised `.app`/`.dmg` is on the roadmap. First start: right-click > Open, or `xattr -d com.apple.quarantine dd-installer`.
