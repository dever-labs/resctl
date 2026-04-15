# resctl

A minimal CLI for managing display resolution on Windows 11.

## Quick start

### Download a release

Download `resctl.exe` from the [Releases](../../releases) page, then run:

```
resctl install
```

This copies the binary to `%USERPROFILE%\bin\resctl.exe` and adds that folder
to your user `PATH`.  Restart your terminal and you're done.

### Build from source

```powershell
git clone https://github.com/dever-labs/resctl
cd resctl
.\build.ps1
.\resctl.exe install
```

---

## Commands

| Command | Description |
|---|---|
| `resctl list` | List all available resolutions for the primary display |
| `resctl get` | Show the current resolution |
| `resctl set <WxH[@Hz]>` | Change resolution |
| `resctl toggle [res1 res2 …]` | Toggle between two (or more) resolutions |
| `resctl install` | Install to `~/bin` and add to `PATH` |
| `resctl uninstall` | Remove from `~/bin` |

## Usage examples

```
# See what your monitor supports
resctl list

# Check current resolution
resctl get

# Switch to 1920×1080 (keeps current refresh rate if supported)
resctl set 1920x1080

# Switch to 2560×1440 at 144 Hz
resctl set 2560x1440@144

# Set up a toggle between two resolutions and switch immediately
resctl toggle 1920x1080 2560x1440

# Subsequent calls cycle through the saved list
resctl toggle
```

The toggle list is saved to `%APPDATA%\resctl\state.json`.

## Uninstall

```
resctl uninstall
```

Then optionally remove `%APPDATA%\resctl` to clean up saved state.
