# macOS

* Code: `macos.go` (build tag `darwin`).
* **Local install is not supported**: Dune Docker needs x86-64 Linux with AVX2, which macOS/Apple Silicon cannot provide. Use the **SSH target** to install on a Linux server.
* Build on a Mac (Xcode command line tools): `./scripts/build.sh` produces arm64 and amd64 archives. The native GUI needs CGO, so it cannot be cross-compiled from Linux.
* Distribution: unsigned binary for now; notarised `.app`/`.dmg` is on the roadmap. First start: right-click > Open, or `xattr -d com.apple.quarantine dd-installer`.
* The archive contains `DD-Installer.app` with the logo as app icon (`.icns` is generated in CI with `sips`/`iconutil`; if that step fails the app is still built without an icon). Unsigned: first start via right-click > Open.
* Look: the Dune-inspired theme is drawn by the toolkit and is identical to the other systems. Retina displays are supported by the toolkit but not yet verified on a real Mac.
