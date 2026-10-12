# resctl

[![CI](https://github.com/dever-labs/resctl/actions/workflows/ci.yml/badge.svg)](https://github.com/dever-labs/resctl/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dever-labs/resctl)](https://github.com/dever-labs/resctl/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dever-labs/resctl)](go.mod)
[![License](https://img.shields.io/github/license/dever-labs/resctl)](LICENSE)

A minimal, single-binary CLI for changing display resolution. No dependencies, no installer — just drop the binary in your PATH and go.

| Platform | Support |
|---|---|
| Windows 7 / 8 / 10 / 11 | ✅ via Win32 API |
| Linux amd64 (X11) | ✅ via xrandr |
| Linux amd64 (Wayland) | ✅ via zwlr-output-management protocol |
| Linux arm64 (X11 + Wayland) | ✅ Raspberry Pi 4/5, Asahi Linux, ARM VMs |
| macOS | ✅ via CoreGraphics |

---

## Installation

### Windows

#### Option 1 — Download a release (recommended)

1. Grab `resctl.exe` from the [latest release](https://github.com/dever-labs/resctl/releases/latest).
2. Optionally verify the download against `checksums.txt` (also in the release assets):

   ```powershell
   # PowerShell
   $hash = (Get-FileHash resctl.exe -Algorithm SHA256).Hash.ToLower()
   $expected = (Select-String resctl.exe checksums.txt).Line.Split()[0]
   if ($hash -eq $expected) { "OK" } else { "MISMATCH" }
   ```

3. Open a terminal **in the folder containing the download** and run:

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

### Linux

> **X11 requires** `xrandr` (`x11-xserver-utils` on Debian/Ubuntu, `xorg-xrandr` on Arch).  
> **Wayland** works out of the box — the binary speaks the `zwlr-output-management-unstable-v1` protocol directly. No extra packages needed. Requires a compositor that implements that protocol (e.g. sway, wayfire, labwc, or any wlroots-based compositor). **GNOME (Mutter) and KDE (KWin) do not implement this protocol** — `resctl` detects this and tells you to use `gnome-randr`/Settings on GNOME, `kscreen-doctor` on KDE, or an `xrandr`-based tool via XWayland elsewhere.

#### Option 1 — Download a release (recommended)

1. Grab the binary for your architecture from the [latest release](https://github.com/dever-labs/resctl/releases/latest):
   - `resctl` — amd64 (most desktops/laptops)
   - `resctl-linux-arm64` — arm64 (Raspberry Pi 4/5, Asahi Linux, ARM cloud VMs)
2. Optionally verify the download against `checksums.txt` (also in the release assets):

   ```sh
   sha256sum -c checksums.txt --ignore-missing
   # resctl: OK
   ```

3. Run:

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

### macOS

#### Option 1 — Download a release (recommended)

1. Grab `resctl-macos` from the [latest release](https://github.com/dever-labs/resctl/releases/latest).
2. Optionally verify the download against `checksums.txt` (also in the release assets):

   ```sh
   sha256sum -c checksums.txt --ignore-missing
   # resctl-macos: OK
   ```

3. Run:

   ```sh
   mv resctl-macos resctl
   chmod +x resctl
   ./resctl install
   ```

   This copies the binary to `~/bin/resctl` and adds `~/bin` to your `PATH` in `.bash_profile`, `.zshrc`, `.config/fish/config.fish`, or `.profile`.

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
resctl uninstall           # prompts for confirmation
resctl uninstall --force   # skip the confirmation prompt
```

---

## Usage

```
resctl list                    List all supported resolutions (current marked with *)
resctl get                     Show the current resolution
resctl set <WxH[@Hz]> [--force]  Change resolution (skips the change if already active, unless --force)
resctl toggle [res1 res2 ...]  Cycle between resolutions
resctl toggle --list           Show the saved toggle list and current position
resctl toggle --clear          Delete the saved toggle list
resctl install                 Copy binary to ~/bin and add to PATH
resctl uninstall [--force]     Remove binary from ~/bin (prompts for confirmation)
resctl completion <shell>      Print a completion script (bash, zsh, fish)
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

# No-op: already at 1920x1080, prints "Already at 1920x1080 @ 60Hz" and exits
resctl set 1920x1080

# Reapply the same resolution anyway (e.g. to force a mode re-negotiation)
resctl set 1920x1080 --force

# Set up a two-resolution toggle and switch immediately
resctl toggle 1920x1080 2560x1440

# After that, each call to `toggle` cycles to the next entry
resctl toggle

# Inspect the saved toggle list and current position
resctl toggle --list
# Toggle list:
#   [0] 1920x1080
#   [1] 2560x1440  ← current

# Delete the saved toggle list
resctl toggle --clear

# List all connected displays and their current resolution
resctl displays
# Connected displays:
#   DP-1: 2560x1440 @ 144Hz
#   HDMI-1: 1920x1080 @ 60Hz

# Enable shell completion
resctl completion bash > /etc/bash_completion.d/resctl
resctl completion zsh > "${fpath[1]}/_resctl"
resctl completion fish > ~/.config/fish/completions/resctl.fish
```

**Toggle state** is saved and persists across reboots:
- Windows: `%APPDATA%\resctl\state.json`
- Linux: `$XDG_CONFIG_HOME/resctl/state.json` (defaults to `~/.config/resctl/state.json`)
- macOS: `~/Library/Application Support/resctl/state.json`

---

## How it works

- **Windows** — calls `EnumDisplaySettingsW` and `ChangeDisplaySettingsW` directly. No admin rights required for the primary display.
- **Linux (X11)** — shells out to `xrandr`. Targets the primary connected output. Requires an X11 session.
- **Linux (Wayland)** — connects to the Wayland compositor socket and speaks the `zwlr-output-management-unstable-v1` protocol natively. No external tools required. The session is detected automatically via `WAYLAND_DISPLAY` or `XDG_SESSION_TYPE`.
- **macOS** — calls CoreGraphics (`CGDisplayCopyDisplayMode`, `CGDisplayCopyAllDisplayModes`, and `CGDisplaySetDisplayMode`) directly for the main display. No external tools required.

---

## Development

The easiest way to get started is with the included **Dev Container** — open the repo in VS Code or GitHub Codespaces and everything is set up automatically.

### Requirements (without Dev Container)

- Go 1.26+
- **Windows build:** `goversioninfo` (`go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`)
- **Linux build:** `xrandr` for smoke-testing
- **macOS build:** Xcode Command Line Tools for cgo and CoreGraphics

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

**macOS:**
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

```sh
# macOS
go test ./...
```

### Release

Tag the commit you want to release, then create a release from the GitHub UI and attach the built binary as an asset:

```powershell
$env:VERSION = "v1.0.0"
.\build.ps1
```

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for branch naming, commit conventions, and the local verification checklist expected before opening a PR.

---

## Troubleshooting

**`xrandr: command not found` / `xrandr query failed`**
`resctl` shells out to `xrandr` on X11. Install it: `sudo apt install x11-xserver-utils` (Debian/Ubuntu) or `sudo pacman -S xorg-xrandr` (Arch). This error can also appear if you're actually in a Wayland session but `xrandr` (XWayland-aware) still fails — check `echo $XDG_SESSION_TYPE` / `echo $WAYLAND_DISPLAY`.

**`zwlr_output_manager_v1 not available` on GNOME or KDE**
GNOME (Mutter) and KDE (KWin) don't implement the `zwlr-output-management` protocol that `resctl` uses natively on Wayland. Use `gnome-randr` or Settings → Displays on GNOME, or `kscreen-doctor` on KDE. On other desktops, the error message suggests trying an `xrandr`-based tool via XWayland. See [#7](https://github.com/dever-labs/resctl/issues/7) for background.

**`cannot connect to Wayland socket` / `XDG_RUNTIME_DIR is not set`**
`resctl` needs `XDG_RUNTIME_DIR` and `WAYLAND_DISPLAY` set, which is normally done automatically by your session manager. This usually means you're running `resctl` from a context without a graphical session attached (e.g. an SSH session, a `sudo`-elevated shell that dropped the environment, or a `cron` job) — run it from within your graphical session instead.

**`resctl install` updated PATH but `resctl` still isn't found**
Restart your terminal (or re-source your shell rc file, e.g. `source ~/.bashrc`) — PATH changes only take effect in new shell sessions. If that doesn't help, check that your `SHELL` environment variable actually points at your real shell (`echo $SHELL`); `resctl install` picks the rc file to edit based on it, and an unusual/misconfigured `SHELL` can cause it to fall back to `~/.profile` instead of `~/.bashrc`/`~/.zshrc`/`~/.config/fish/config.fish`.

**`display "X" not found` when using `--display`**
Display names are platform-specific (e.g. `DP-1`, `HDMI-1` on Linux; a numeric index on Windows/macOS). Run `resctl list` without `--display` first to see the primary display's modes, or check your compositor/X11 output name directly (`xrandr --query` on X11, or your Wayland compositor's own tooling).

---

## License

[MIT](LICENSE) © Dever Labs
