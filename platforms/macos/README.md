# macOS

* Code: `macos.go` (build tag `darwin`).
* **Local install is not supported**: Dune Docker needs x86-64 Linux with AVX2, which macOS/Apple Silicon cannot provide. Use the **SSH target** to install on a Linux server.
* Build on a Mac (Xcode command line tools): `./scripts/build.sh` produces arm64 and amd64 archives. The native GUI needs CGO, so it cannot be cross-compiled from Linux.
* Distribution: unsigned binary for now; notarised `.app`/`.dmg` is on the roadmap. First start: right-click > Open, or `xattr -d com.apple.quarantine dd-installer`.
