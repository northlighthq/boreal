# Changelog

All notable changes to Boreal (formerly Platty) are listed here. Versions follow [Semantic Versioning](https://semver.org).

## 0.1.1-beta.4 (2026-10-29)

Platty is now Boreal, with a new interface built around an activity bar.

### Boreal
- Platty is now Boreal, part of Northlight: new name, the Northlight mark and app icon, and a Northlight theme with the Aurora accent as the default.
- Settings and the vault move from `%AppData%\Platty` to `%AppData%\Boreal` on first launch. Exports use the `.boreal` extension; `.platty` files still import.
- Updates keep working: releases also ship the old file names for versions that still look for them.
- The installer moves Boreal to its own folder and removes the old Platty program and shortcuts.

### Interface
- An activity bar on the left opens hosts, SFTP, snippets, keys, port forwarding and known hosts, with Lucide icons.
- A minimal home screen with quick actions, recent hosts and the command list.
- Passwords and passphrases are asked inside the tab that needs them, and the tab shows a lock while it waits.
- New connecting and failed screens that list each step as it happens.
- Search Everywhere opens centered over the content.

### Windows on ARM
- Native ARM64 builds: an installer and a portable `.zip` next to the x64 ones.
- The updater downloads the build that matches the running architecture.

## 0.1.1-beta.2 (2026-10-16)

Choose which panes receive broadcast input, so test and production sessions can share a tab safely.

### Terminal
- Broadcast input can leave panes out: each pane shows whether it is receiving, and clicking that label (or the pane menu) includes or excludes it. Panes added later start excluded.
- Ctrl+Shift+B starts and stops broadcasting in the current tab.
- Hosts tagged `prod` or `production` start out of the broadcast and are marked as protected; the tag list is in Settings › Terminal.

### Interface
- Applying settings confirms the change with a notification.
- Folders in the host list no longer show a host count.

## 0.1.1-beta.1 (2026-10-15)

A security and performance beta, and the first build that updates itself.

### Updates
- Platty checks GitHub for new versions once a day and offers them in the sidebar and in Settings › About.
- Updates are verified against a signed checksum list before they install; installed and portable builds both update in place.

### Security
- Ctrl+click only opens http and https links, so a program can't make Platty run a file or a Windows protocol handler.
- SFTP downloads clean up file names sent by the server, so they can't be written outside the chosen folder, and editing a remote program opens it in Notepad instead of running it.
- Programs in the terminal now ask before copying to the clipboard (Settings › Terminal).
- Pasted text can no longer end bracketed paste early and run commands on its own.
- Fixed crashes from wide characters in one-column panes and from malformed server metrics; an internal terminal error now resets that tab instead of closing Platty.
- Exporting to `~/.ssh/config` strips characters that could add directives, and settings and the vault are flushed to disk before they replace the old file.

### Performance
- The terminal uploads only the rows that change, reuses rows when scrolling and skips frames where nothing changed.
- Split panes share one glyph cache, resizing waits until you stop dragging, and terminal cells use 20% less memory.
- Server metrics are only fetched for the visible pane.

## 0.1.0 (2026-10-14)

The first public beta.

### Terminal
- Own VT emulator with truecolor, alternate screen, scrollback, wide characters and reflow on resize.
- Custom glyph rasterizer with color emoji, font fallback and ligatures (JetBrains Mono, Fira Code, Geist Mono).
- Tabs and split panes that come back when Platty reopens; rename, reorder and duplicate tabs.
- Broadcast input to every pane of a tab, and plain-text session logs.
- Search in the scrollback, Ctrl+click links (including OSC 8), OSC 52 clipboard writes and OSC 7 working folder.

### SSH
- Hosts in folders with drag and drop, tags, search and `~/.ssh/config` import and export.
- Password, key, agent and keyboard-interactive authentication; jump hosts, keep-alive and reconnect.
- Host key verification with a warning when a key changes.
- Keychain with ed25519 key generation, import, SSH certificates, and passphrases kept per session unless remembered.
- Agent forwarding and a command to run on connect, per host.
- Local, remote and dynamic (SOCKS5) port forwarding.
- SFTP panel with a transfer queue, folder transfers, remote editing and drag and drop in both directions.
- Server metrics for SSH sessions: CPU, memory, disk, network, load and uptime.

### Security and data
- Vault sealed with XChaCha20-Poly1305; the key is wrapped with DPAPI and/or an Argon2id master password.
- Lock on demand (Ctrl+Shift+L) or after a period of inactivity.
- Encrypted `.platty` export and import that never deletes data or replaces trusted host keys.
- Option to hide IP addresses on screen.

### App
- A first-launch introduction with Sol.
- Search Everywhere (Ctrl+Shift+P), four monochrome themes and an About page.
