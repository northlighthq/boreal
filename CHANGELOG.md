# Changelog

All notable changes to Boreal (formerly Platty) are listed here. Versions follow [Semantic Versioning](https://semver.org).

## Unreleased

### macOS and Linux
- macOS unlocks Boreal with Touch ID and Linux with a fingerprint reader (fprintd), the way Windows does with Windows Hello.
- On macOS, files dropped from Finder on an SSH session upload via SFTP, and files and folders dragged out of SFTP download where you drop them.
- Locking the Mac or the Linux session (through logind) can lock Boreal too.
- Boreal can open `ssh://` and `sftp://` links on macOS and Linux. Turn it on in Settings, as on Windows.
- Output triggers can play a sound on macOS and Linux.
- On Linux with Wayland, files dropped on an SSH session upload via SFTP.
- On macOS the app and terminal shortcuts also work with ⌘ (⌘T, ⌘W, ⌘C, ⌘V, ⌘F, ⌘, …), ⌘-click opens links and shortcuts are shown with Mac symbols.
- The Linux AppImage downloads and installs its own updates. On macOS, updating opens the release page.

### Android
- A first Android app (Android 9 and later) shares the desktop app's vault format, SSH and terminal. It has hosts, keys, snippets, settings and several sessions at once.
- Open sessions stay connected while you switch apps, with a notification showing how many are open.
- The terminal has a key bar with Esc, Tab, Ctrl, Alt, arrows, Home, End, page keys, F1–F12, Ins and Del. You can scroll it with a flick, copy what's on screen and paste.
- In landscape with the keyboard open, the terminal hides its title bar to keep more lines visible.
- The vault locks again after Boreal has been in the background for the time you choose in Settings.
- Fields for keys, commands and addresses turn off autocorrect and suggestions.

## 0.1.1-beta.8 (2026-11-21)

### MCP
- MCP clients can use Boreal through the Model Context Protocol with `boreal mcp`. It's off until you turn it on in Settings › MCP, which warns before it does, and an imported file can never turn it on.
- Settings › MCP chooses what clients may do (read, change access, run commands, read and write files, start tunnels) and which folders of hosts they can reach, shows the configuration to paste into the client and lists their recent calls.
- Tools list hosts, keys and snippets, show which keys each host's authorized_keys accepts, and add or remove a key with optional limits: source addresses, a single command, an expiry date, no forwarding or no terminal.
- Passwords, private keys, passphrases and proxy commands are never shared. Clients only reach servers whose key you already trusted, back up authorized_keys before each change, and can't remove the key Boreal signs in with.
- With "Run commands" allowed, which asks for confirmation first, clients can run a command or a saved snippet on a shared host and get its exit code and output. Commands run without a terminal, stop after their time limit (30 seconds by default, 5 minutes at most) and return at most 64 KB of output per stream.
- With "Read files" allowed, clients can list folders and read files on a shared host over SFTP, in parts for large files. With "Write files" also allowed, which asks for confirmation first, they can create files of up to 1 MB and replace one only when they ask to. Files are swapped in whole and keep their permissions, and nothing in ~/.ssh or named authorized_keys can be written, even through a link.
- With "Start tunnels" allowed, clients can start and stop the tunnels saved for shared hosts, only those that listen on this computer. They stop after their time limit (an hour by default, 8 hours at most), when the client disconnects and within seconds of turning the permission off.
- If a server's key changed since Boreal last connected, clients are told so and Boreal doesn't connect.
- Every call, including each command run, file path and tunnel, is written to mcp.log in Boreal's folder.

### Security
- When the system keyring isn't available, Boreal asks for a master password instead of saving the vault key unprotected.
- Changing or removing the master password re-encrypts the vault with a new key.
- With Windows Hello turned on, Boreal starts locked and asks for it. Settings › Security explains what Hello protects when there's no master password.
- Host names, addresses and users are quoted when they're inserted into a proxy command.
- Imported files no longer bring proxy commands, commands run on connect, or tunnels that start on their own or listen beyond this computer. Boreal lists what it left out.
- Editing a remote file always opens it in a text editor, whatever its extension.
- The tunnel editor warns when a tunnel listens beyond this computer.
- Pasting several lines asks first when the shell would run them as they arrive.
- The master password needs at least 10 characters everywhere.

### SSH
- Hosts that sign in with the agent can choose which agent to use and which of its keys to offer. With many keys in 1Password or Bitwarden, picking one avoids “Too many authentication failures”.
- On macOS and Linux, Boreal finds the 1Password and Bitwarden agents even when SSH_AUTH_SOCK isn't set, like when it's opened from the Dock.
- IdentityAgent is read when importing ~/.ssh/config and written when exporting it.
- Imported files can't point a host at an agent on another computer.
- Settings › SSH shows which agent Boreal found.

### Terminal
- Ctrl+Backspace deletes the word before the cursor and Ctrl+Delete the word after it, over SSH and in local shells. Full screen programs like vim still get the usual keys.

### Interface
- With more tabs than fit, the tab bar no longer swaps tabs around when you pick a hidden one. It scrolls just enough to show it, and the mouse wheel scrolls it too.
- Closing tabs with the close button keeps the others the same width until the pointer leaves the tab bar, so the next close button stays under it.
- Middle click closes a tab, and the tab menu can close every tab to the right.

## 0.1.1-beta.7 (2026-11-08)

### Terminal
- Triggers in Settings › Triggers highlight, notify or play a sound when terminal output matches a regular expression, for every session, local terminals only or one host. A notification's Show button opens the pane. Lines you type don't count.
- The title of a background tab turns bright when its terminal prints something new, until you open it. Settings › Appearance turns this off.

### Interface
- Notifications are more compact. Their action, like Show or Retry, is a link next to the title, and a trigger notification shows the matched line as it looks in the terminal.

### Fixes
- Boreal can be closed while locked with SSH sessions or tunnels open, instead of getting stuck.

## 0.1.1-beta.6 (2026-11-05)

### Security
- Windows Hello can be turned off in Settings › Security, separately from how the vault is stored.
- Boreal can lock after 1 minute, 2 hours or 4 hours, besides the existing times.
- The lock options come first in Settings › Security.

### Interface
- A new lock screen, with Windows Hello as the main action.

## 0.1.1-beta.5 (2026-11-04)

### Hosts
- Hosts show the logo of their operating system, detected when you connect. Settings › Appearance switches back to the terminal icon and between color and monochrome.
- The host editor shows a key icon next to each SSH key and a menu with your folders next to the Folder field.

### Security
- Built with Go 1.26.10 and updated `golang.org/x/net`, which fix several issues in HTTP, TLS and certificate handling.

### Fixes
- Borders are no longer cut on the left of checkboxes, fields, selects and other controls.
- The whole box of a text field is clickable, not just the text.
- Clicking outside a dialog closes it, like Esc.

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
