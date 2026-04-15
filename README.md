# resctl

[![CI](https://github.com/dever-labs/resctl/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/resctl/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dever-labs/resctl)](https://github.com/dever-labs/resctl/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dever-labs/resctl)](go.mod)
[![License](https://img.shields.io/github/license/dever-labs/resctl)](LICENSE)

A minimal, single-binary CLI for changing display resolution on Windows 11. No dependencies, no installer — just drop the `.exe` in your PATH and go.

---

## Installation

### Option 1 — Download a release (recommended)

1. Grab `resctl.exe` from the [latest release](https://github.com/dever-labs/resctl/releases/latest).
2. Open a terminal **in the folder containing the download** and run:

   ```
   .\resctl.exe install
   ```

   This copies the binary to `%USERPROFILE%\bin\resctl.exe` and adds that folder to your user `PATH`.

3. Restart your terminal — you're done.

### Option 2 — Build from source

```powershell
git clone https://github.com/dever-labs/resctl
cd resctl
.\build.ps1          # installs goversioninfo, embeds resources, builds resctl.exe
.\resctl.exe install
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

```powershell
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

The toggle list is saved to `%APPDATA%\resctl\state.json` and persists across reboots.

---

## How it works

resctl calls the Win32 `EnumDisplaySettingsW` and `ChangeDisplaySettingsW` APIs directly — no third-party dependencies, no admin rights required for the primary display.

---

## Development

### Requirements

- Go 1.21+
- Windows (the tool uses Windows-only APIs)

### Build

```powershell
.\build.ps1
```

Or manually:

```powershell
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
goversioninfo -64 -o resource.syso   # embed icon & version metadata
go build -ldflags="-s -w -X main.version=dev" -o resctl.exe .
```

### Test

Tests run *without* `resource.syso` present (the test linker cannot handle Windows COFF resource objects):

```powershell
Remove-Item resource.syso -ErrorAction SilentlyContinue
go test ./...
```

### Release

Push a `v*` tag to trigger the release pipeline:

```
git tag v1.0.0
git push origin v1.0.0
```

The CI will run tests, build the binary with the tag embedded as the version string, and publish a GitHub Release with `resctl.exe` attached as a downloadable asset.

---

## License

[MIT](LICENSE) © Dever Labs
