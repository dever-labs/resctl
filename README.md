# resctl

[![CI](https://github.com/dever-labs/resctl/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/resctl/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dever-labs/resctl)](https://github.com/dever-labs/resctl/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dever-labs/resctl)](go.mod)
[![License](https://img.shields.io/github/license/dever-labs/resctl)](LICENSE)

A minimal, single-binary CLI for changing display resolution. No dependencies, no installer — just drop the binary in your PATH and go.

| Platform | Support |
|---|---|
| Windows 7 / 8 / 10 / 11 | ✅ via Win32 API |
| Linux (X11) | ✅ via xrandr |
| Linux (Wayland) | ❌ not supported |

---

## Installation

### Windows

#### Option 1 — Download a release (recommended)

1. Grab `resctl.exe` from the [latest release](https://github.com/dever-labs/resctl/releases/latest).
2. Open a terminal **in the folder containing the download** and run:

   ```
   .\resctl.exe install
   ```

   This copies the binary to `%USERPROFILE%\bin\resctl.exe` and adds that folder to your user `PATH`.

3. Restart your terminal — you're done.

#### Option 2 — Build from source

```powershell
git clone https://github.com/dever-labs/resctl
cd resctl
.\build.ps1          # installs goversioninfo, embeds resources, builds resctl.exe
.\resctl.exe install
```

### Linux (X11)

> **Requires** `xrandr` (`x11-xserver-utils` on Debian/Ubuntu, `xorg-xrandr` on Arch).

#### Option 1 — Download a release (recommended)

1. Grab `resctl` from the [latest release](https://github.com/dever-labs/resctl/releases/latest).
2. Run:

   ```sh
   chmod +x resctl
   ./resctl install
   ```

   This copies the binary to `~/bin/resctl` and adds `~/bin` to your `PATH` in `.bashrc`, `.zshrc`, or `.profile`.

3. Restart your terminal — you're done.

#### Option 2 — Build from source

```sh
git clone https://github.com/dever-labs/resctl
cd resctl
go build -ldflags="-s -w" -o resctl .
./resctl install
```

### Uninstall

```
resctl uninstall
```

---

## Usage

```
resctl list                    List all supported resolutions (current marked with *)
resctl get                     Show the current resolution
resctl set <WxH[@Hz]>          Change resolution
resctl toggle [res1 res2 ...]  Cycle between resolutions
resctl install                 Copy binary to ~/bin and add to PATH
resctl uninstall               Remove binary from ~/bin
resctl version                 Print the installed version
```

**Resolution format:** `WxH` or `WxH@Hz` — e.g. `1920x1080` or `2560x1440@144`.

### Examples

```sh
# See everything your monitor supports
resctl list

# What's active right now?
resctl get

# Switch to 1080p (keeps the current refresh rate if available)
resctl set 1920x1080

# Switch to 1440p at 144 Hz explicitly
resctl set 2560x1440@144

# Set up a two-resolution toggle and switch immediately
resctl toggle 1920x1080 2560x1440

# After that, each call to `toggle` cycles to the next entry
resctl toggle
```

**Toggle state** is saved and persists across reboots:
- Windows: `%APPDATA%\resctl\state.json`
- Linux: `$XDG_CONFIG_HOME/resctl/state.json` (defaults to `~/.config/resctl/state.json`)

---

## How it works

- **Windows** — calls `EnumDisplaySettingsW` and `ChangeDisplaySettingsW` directly. No admin rights required for the primary display.
- **Linux** — shells out to `xrandr`. Targets the primary connected output. Requires an X11 session.

---

## Development

The easiest way to get started is with the included **Dev Container** — open the repo in VS Code or GitHub Codespaces and everything is set up automatically.

### Requirements (without Dev Container)

- Go 1.20+
- **Windows build:** `goversioninfo` (`go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`)
- **Linux build:** `xrandr` for smoke-testing

### Build

**Windows:**
```powershell
.\build.ps1
```

Or manually:
```powershell
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
goversioninfo -64 -o resource.syso   # embed icon & version metadata
go build -ldflags="-s -w -X main.version=dev" -o resctl.exe .
```

**Linux:**
```sh
go build -ldflags="-s -w -X main.version=dev" -o resctl .
```

**Cross-compile for Windows from Linux (Dev Container):**
```sh
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.version=dev" -o resctl.exe .
```

### Test

Tests run *without* `resource.syso` present (the test linker cannot handle Windows COFF resource objects):

```powershell
# Windows
Remove-Item resource.syso -ErrorAction SilentlyContinue
go test ./...
```

```sh
# Linux / Dev Container
go test ./...
```

### Release

Tag the commit you want to release, then create a release from the GitHub UI and attach the built binary as an asset:

```powershell
$env:VERSION = "v1.0.0"
.\build.ps1
```

---

## License

[MIT](LICENSE) © Dever Labs
