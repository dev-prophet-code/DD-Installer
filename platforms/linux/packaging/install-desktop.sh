#!/bin/sh
# Adds DD-Installer to your application menu (current user only).
# Run it from the extracted folder:  ./install-desktop.sh
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
icons="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/512x512/apps"
apps="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
mkdir -p "$icons" "$apps"
cp "$here/dd-installer.png" "$icons/dd-installer.png"
sed "s|@EXEC@|$here/dd-installer|" "$here/dd-installer.desktop" > "$apps/dd-installer.desktop"
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$apps" || true
echo "Installed. Look for 'Dune Docker Installer' in your application menu."
