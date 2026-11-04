#!/usr/bin/env bash
# Prints the notes of a release: its CHANGELOG section plus how to install it.
set -euo pipefail

version="${1:?usage: release-notes.sh <version>}"
root="$(cd "$(dirname "$0")/.." && pwd)"

section() {
  awk -v v="$1" '/^## / { if (on) exit; on = ($2 == v); next } on' "$root/CHANGELOG.md"
}

notes="$(section Unreleased)"

if ! grep -q '[^[:space:]]' <<<"$notes"; then
  notes="$(section "$version")"
fi

if ! grep -q '[^[:space:]]' <<<"$notes"; then
  echo "CHANGELOG.md has nothing under \"## Unreleased\" or \"## $version\"" >&2
  exit 1
fi

cat <<NOTES
$notes

### Install

**Windows**
- **boreal-$version-setup.exe** installs Boreal for your user, without administrator rights.
- **boreal-$version-windows-amd64.zip** is the portable version: unzip it and run \`boreal.exe\`.
- On Windows on ARM (Snapdragon, Surface Pro X), use **boreal-$version-arm64-setup.exe** or **boreal-$version-windows-arm64.zip**.

Beta builds aren't code-signed yet. If SmartScreen stops the download or the first launch, choose *More info* and then *Run anyway*.

**macOS** (12 or later)
- Use **boreal-$version-macos-apple-silicon.dmg** on M1 and newer, **boreal-$version-macos-intel.dmg** on older Macs.
- Drag Boreal to Applications. It isn't notarized yet: the first time, open it from System Settings › Privacy & Security › Open Anyway.

**Linux**
- Use **boreal-$version-linux-x86_64.AppImage** on most PCs, **boreal-$version-linux-aarch64.AppImage** on ARM.
- Run \`chmod +x\` on the file and open it. It needs Wayland or X11 with OpenGL ES or Vulkan. Without FUSE, add \`--appimage-extract-and-run\`.
- Without a system keyring (GNOME Keyring, KWallet), set a master password in Settings › Security.

Automatic updates are only available on Windows for now. On macOS and Linux, download new versions from this page.
NOTES
