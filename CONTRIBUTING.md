# Contributing to resctl

Thanks for your interest in contributing! This document covers everything you need to get set up and submit a change.

## Getting started

The fastest way to get a working environment is the provided Dev Container (`.devcontainer/`), which already has Go, `xrandr`, and the other tools you need. If you're not using the Dev Container, see [Development → Requirements](README.md#requirements-without-dev-container) in the README.

```sh
git clone https://github.com/dever-labs/resctl.git
cd resctl
go build ./...
go test ./...
```

## Making a change

1. Create a branch off `main` — use a short, descriptive name with a type prefix, e.g. `feat/display-list-command` or `fix/toggle-race`. Don't prefix branch names with your username.
2. Make your change. Keep PRs focused — one logical change per PR.
3. Run the full local verification before opening a PR:

   ```sh
   go build ./...
   go vet ./...
   go test ./...
   gofmt -l .          # should print nothing
   golangci-lint run ./...
   ```

4. Commit using [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, `test:`, `ci:`, `docs:`). This repo uses [release-please](https://github.com/googleapis/release-please), which parses commit types to generate the changelog and version bump — so an inaccurate type will show up in the release notes.
5. Push and open a PR. CI runs lint/test/build across Windows, Linux, and macOS, plus CodeQL analysis — all must pass before merge.
6. PRs are merged via **squash merge** (the repo doesn't allow merge commits or rebase merge), so your branch's commit history is collapsed into one commit using the PR title/description — write a clear PR title and description accordingly.

## Code conventions

- Platform-specific logic lives in `_windows.go` / `_linux.go` / `_darwin.go` files with matching `//go:build` tags. The Windows variant (no suffix, e.g. `config.go`, `display.go`) is the default/un-suffixed build target.
- Linux has two display backends: `xrandr` (X11) and a native `zwlr-output-management-unstable-v1` Wayland client (`wayland_linux.go`) with no external dependencies. `isWayland()` in `display_linux.go` decides which one to use.
- Keep pure/testable logic (parsing, decision helpers) separate from code that touches OS display APIs, so it can be unit tested without real hardware. Functions like `isNoOp`, `groupModesByResolution`, and `parseXrandrOutput` are good examples.
- All third-party GitHub Actions are pinned to a full commit SHA with a `# vX.Y.Z` comment — not a floating tag.

## Reporting bugs / requesting features

Please open a GitHub issue using the provided templates. Include your OS/display server (X11 or Wayland, and compositor if Wayland) when reporting a bug — most issues in this project are platform-specific.

## Questions

Open a [discussion](https://github.com/dever-labs/resctl/discussions) or an issue — whichever fits best.
