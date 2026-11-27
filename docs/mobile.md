# Mobile app guidelines

Boreal on Android has its own interface in `internal/mobile`. It shares the SSH layer, the vault and
the terminal engine with the desktop app, but never its layout. This page is the checklist every
mobile screen follows.

## Layout of the code

| Folder | What goes there |
|---|---|
| `internal/core` | No UI. Sessions, SSH, the vault, tunnels, settings. Used by desktop and mobile. |
| `internal/common` | Gio widgets shared by both apps: the component kit, the terminal view, themes, icons. |
| `internal/desktop` | The desktop window. Nothing in `internal/mobile` imports it. |
| `internal/mobile` | The Android app. `ui` is its design system, `nav` the screen stack, and each feature has its own package (`hosts`, `terminal`, `keys`, `snippets`, `settings`, `lock`). |
| `internal/platform` | Code for one operating system, picked with build tags. `android` holds the JNI bridge and its Java classes. |

Go treats Android as Linux, so a `_linux.go` file also builds for Android. Files that must not run
there carry `//go:build linux && !android`.

## Screens

- Every screen implements `nav.Screen`. Optional interfaces add a subtitle (`ui.Subtitled`), top bar
  actions (`ui.Actionable`) or a full screen layout without bars (`ui.FullScreen`).
- Root screens use the large title and the bottom navigation. Pushed screens get a back arrow and
  hide the bottom navigation.
- Creating or editing something opens a full screen with its own back arrow, never a side panel or
  a floating window.
- A screen never builds its own dialogs or sheets. It calls `Host.Ask` and `Host.ShowSheet` so the
  Android back button can close them.

## Touch

- Anything tappable is at least 48 dp tall. Use `ui.Row`, `ui.Button`, `ui.IconButton` and `ui.FAB`.
- Nothing depends on hover or right click. Extra actions go in a `⋮` button that opens a sheet.
- Pressed states replace hover: rows and buttons darken while the finger is down.
- Keyboard shortcuts and their hints (`Ctrl+Shift+P`) never appear in the mobile UI.
- Text fields use the right keyboard: `key.HintNumeric` for ports, `HintURL` for addresses,
  `HintPassword` for secrets.

## System integration

- The UI stays inside the window insets: nothing under the status bar, the gesture bar or the
  keyboard. The app paints the bars with the theme colors.
- The back button closes a dialog, then a sheet, then the current screen, then goes to Hosts, and
  only then leaves the app. Moving to another screen hides the keyboard.
- Builds target the latest Android SDK. The back button works through
  `OnBackInvokedDispatcher` (`internal/platform/android/BackHandler.java`), since Android 16 no
  longer calls `onBackPressed`.
- The vault key is sealed by the Android Keystore (`Keystore.java`). It never touches the disk in
  the clear.

## Writing

- Short sentences, plain words, no jargon the screen doesn't need.
- Errors say what happened and what to do next: "Use a number between 1 and 65535."
- Buttons name the action: "Save host", "Generate key", "Trust and connect".

## Before a change is done

1. `go vet ./...` and `go test ./...` pass.
2. The APK builds: `packaging/android/build.ps1`. CI builds it on every pull request.
3. Changes to the Java classes are rebuilt into `boreal_android.jar` with
   `internal/platform/android/build.ps1`.
4. The screen is checked in the emulator, in a dark and a light theme, with and without the
   keyboard open.

## Running it

```sh
# Desktop window with the mobile UI, for quick iteration. It keeps its data in its own
# "Boreal Mobile Preview" folder, so it never opens or changes the desktop vault.
BOREAL_MOBILE=1 go run ./cmd/boreal

# APK for a device or the emulator
./packaging/android/build.ps1 -Version 0.1.1-dev
adb install -r dist/boreal-0.1.1-dev-android.apk
```
