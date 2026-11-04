# Linux

* Code: `linux.go` (build tag `linux`). Local install runs the upstream installer natively via `sh`.
* Requirements: regular user with sudo (the installer refuses root), glibc/musl independent static binary.
* Build: `sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev && ./scripts/build.sh` (CGO; amd64).
* Packaging ideas: tar.gz (built by `scripts/build.sh`), AppImage/.deb later.
* Needs a desktop session (X11/Wayland with OpenGL). For servers without a desktop, run the installer on your own computer and use the SSH target.
* The archive contains `dd-installer.png`, a `.desktop` file and `install-desktop.sh`, which adds a menu entry with the logo for the current user.
* Look: the Dune-inspired theme is drawn by the toolkit and is identical to the other systems; it needs OpenGL (a normal desktop session has it).
