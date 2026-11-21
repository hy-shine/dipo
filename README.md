<p align="center">
  <img src="resources/icon.png" width="96" alt="Dipo logo">
</p>

<h1 align="center">Dipo</h1>

<p align="center"><strong>A local-first color picker for macOS.</strong></p>

<p align="center">
  English | <a href="README.zh-Hans.md">简体中文</a> | <a href="README.zh-Hant.md">繁體中文</a>
</p>

Dipo is a focused color picker for macOS. It is built for quick, repeated use, with no account or cloud sync.

## Features

- Pick from a frozen, magnified screen view; use the arrow keys to move one captured pixel at a time.
- Copy HEX, CSS RGB, or CSS HSL, with a configurable global pick shortcut.
- Edit in RGB, HSB, or HSL, including opacity.
- Check text/background contrast against WCAG AA thresholds.
- Keep the 20 most recent colors and a separate local favorites list.
- Choose in Settings whether Dipo opens at login, shows its menu bar icon, and copies a color right after a pick.

## Install

Download the disk image from the [latest release](https://github.com/hy-shine/dipo/releases/latest), open it, and drag Dipo into Applications. It needs macOS 14 or later, and the release is for Apple silicon. A `.sha256` file next to the disk image holds its checksum. Dipo is not notarized, so see [Permissions](#permissions) for the first launch. Once installed, **Settings → About → Check Now** looks for a newer version.

## Quick start

### Requirements

- macOS 14 or later
- Apple silicon for the current build configuration
- Go 1.27.3 or later
- Xcode Command Line Tools for the Swift screen picker

### Build and run

```sh
make run
```

`make run` builds and launches Dipo. To build without launching it, run `make build`; the app is placed under `build/nsgo/`.

The build bundles the native screen picker. The app is signed ad hoc unless `CODESIGN_IDENTITY` names a code signing identity in your keychain, for example `make run CODESIGN_IDENTITY="<identity name>"`. Keeping one stable identity avoids re-granting Screen Recording after each rebuild.

## Use

1. Select **Pick**, or use the global shortcut (default `Ctrl+Alt+C`; `Control+Option+C` on macOS). Click a pixel to pick it; press Esc or right-click to cancel. The color is copied in the selected format unless **Copy After Picking** is off in Settings.
2. Use **Copy** for the selected format. Recent and favorite colors can be copied by clicking them.
3. Switch between **Contrast**, **Recent**, and **Favorites** below the editor. Recent colors show a count and can be cleared without deleting favorites.
4. Open **Settings** (the top-right gear, or `⌘,`) for login, menu bar, copy and shortcut options. Under **About**, **Check Now** looks for a newer version.

### Permissions

Dipo is not notarized, so macOS may block it on first launch. Try opening it, then allow it in **System Settings → Privacy & Security → Open Anyway**. If the app is installed in `/Applications` and you trust this copy, you can remove the quarantine attribute from Terminal:

```sh
sudo xattr -rd com.apple.quarantine /Applications/Dipo.app
```

If you installed Dipo elsewhere, use that location instead. Only do this for a copy you built or trust. Removing quarantine does not grant Screen Recording access.

The first pick needs macOS Screen Recording permission. Dipo uses the captured frame only for that pick; it is not written to history or uploaded. If access is missing, open the top-right gear and use **Check again** or **Open System Settings**.

## Development

```sh
make check
make check-ui
```

`make check` runs the native picker checks, `go vet`, and Go tests. `make check-ui` runs the NSGo macOS interface tests and saves screenshots under `.nsgo/test/screenshots/macos/`.

## Project layout

| Path | Purpose |
| --- | --- |
| `main.go`, `panel.go`, `settings.go` | NSGo app, main window, and settings window |
| `sampler.go`, `native/` | Go-to-Swift screen picker bridge and macOS capture view |
| `internal/colour/` | Color parsing, formatting, models, and contrast calculations |
| `internal/library/` | Local preferences and recent/favorite color persistence |
| `locales/` | English, Simplified Chinese, and Traditional Chinese catalogs |
| `resources/` | App and menu-bar icons plus the built picker helper |
| `nsgo.yaml`, `Makefile` | App id, version and update key; build, check and release commands |
| `.github/workflows/` | CI for every push, and the release for a version tag |

The app is built with [NSGo](https://github.com/nsgo-dev/nsgo).

## License

Released under the [MIT License](LICENSE).
