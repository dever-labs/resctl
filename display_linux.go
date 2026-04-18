//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Resolution holds a display mode.
type Resolution struct {
	Width  uint32
	Height uint32
	Freq   uint32
}

func (r Resolution) String() string {
	return fmt.Sprintf("%dx%d@%dHz", r.Width, r.Height, r.Freq)
}

// isWayland reports whether the current session is a Wayland session.
func isWayland() bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	return strings.ToLower(os.Getenv("XDG_SESSION_TYPE")) == "wayland"
}

// --- xrandr backend (X11) ---

// xrandrQuery runs xrandr --query and returns the output.
func xrandrQuery() (string, error) {
	out, err := exec.Command("xrandr", "--query").Output()
	if err != nil {
		return "", fmt.Errorf("xrandr query failed: %w (is xrandr installed and running under X11?)", err)
	}
	return string(out), nil
}

// parseXrandrOutput parses xrandr --query output into available modes,
// the current resolution, and the primary output name.
// It targets the output marked as "primary", falling back to the first connected output.
func parseXrandrOutput(output string) (modes []Resolution, current Resolution, outputName string, err error) {
	lines := strings.Split(output, "\n")

	// Find the primary or first connected output.
	startLine := -1
	for i, line := range lines {
		if !strings.Contains(line, " connected") {
			continue
		}
		if startLine == -1 {
			startLine = i // first connected – use as fallback
		}
		if strings.Contains(line, " connected primary") {
			startLine = i // prefer primary
			break
		}
	}
	if startLine == -1 {
		return nil, Resolution{}, "", fmt.Errorf("no connected display found in xrandr output")
	}

	fields := strings.Fields(lines[startLine])
	if len(fields) == 0 {
		return nil, Resolution{}, "", fmt.Errorf("could not parse xrandr output line")
	}
	outputName = fields[0]

	// Parse mode lines (indented lines immediately after the output header).
	seen := make(map[string]bool)
	for _, line := range lines[startLine+1:] {
		if line == "" {
			continue
		}
		if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
			break // reached next output section
		}
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// First token is WxH.
		dims := strings.SplitN(parts[0], "x", 2)
		if len(dims) != 2 {
			continue
		}
		w, werr := strconv.ParseUint(dims[0], 10, 32)
		h, herr := strconv.ParseUint(dims[1], 10, 32)
		if werr != nil || herr != nil {
			continue
		}

		// Remaining tokens are refresh rates; current rate is suffixed with '*'.
		for _, tok := range parts[1:] {
			isCurrent := strings.Contains(tok, "*")
			tok = strings.TrimRight(tok, "*+")
			f, ferr := strconv.ParseFloat(tok, 64)
			if ferr != nil {
				continue
			}
			freq := uint32(f + 0.5) // round to nearest Hz
			key := fmt.Sprintf("%dx%d@%d", w, h, freq)
			if !seen[key] {
				seen[key] = true
				res := Resolution{Width: uint32(w), Height: uint32(h), Freq: freq}
				modes = append(modes, res)
				if isCurrent {
					current = res
				}
			}
		}
	}

	sort.Slice(modes, func(i, j int) bool {
		a, b := modes[i], modes[j]
		if a.Width != b.Width {
			return a.Width < b.Width
		}
		if a.Height != b.Height {
			return a.Height < b.Height
		}
		return a.Freq < b.Freq
	})

	return modes, current, outputName, nil
}

// --- public API ---

// GetCurrent returns the primary display's current resolution.
func GetCurrent() (Resolution, error) {
	if isWayland() {
		_, current, _, err := wlrNativeQuery()
		if err != nil {
			return Resolution{}, err
		}
		if current.Width == 0 {
			return Resolution{}, fmt.Errorf("could not determine current resolution")
		}
		return current, nil
	}

	raw, err := xrandrQuery()
	if err != nil {
		return Resolution{}, err
	}
	_, current, _, err := parseXrandrOutput(raw)
	if err != nil {
		return Resolution{}, err
	}
	if current.Width == 0 {
		return Resolution{}, fmt.Errorf("could not determine current resolution from xrandr output")
	}
	return current, nil
}

// ListModes returns all available display modes for the primary monitor.
func ListModes() ([]Resolution, error) {
	if isWayland() {
		modes, _, _, err := wlrNativeQuery()
		return modes, err
	}

	raw, err := xrandrQuery()
	if err != nil {
		return nil, err
	}
	modes, _, _, err := parseXrandrOutput(raw)
	return modes, err
}

// SetResolution changes the primary display resolution.
func SetResolution(width, height, freq uint32) (Resolution, error) {
	if isWayland() {
		return setResolutionWayland(width, height, freq)
	}
	return setResolutionX11(width, height, freq)
}

func setResolutionWayland(width, height, freq uint32) (Resolution, error) {
	if freq == 0 {
		if resolved, err := pickFreq(width, height); err == nil {
			freq = resolved
		}
	}
	return wlrNativeSet(width, height, freq)
}

func setResolutionX11(width, height, freq uint32) (Resolution, error) {
	raw, err := xrandrQuery()
	if err != nil {
		return Resolution{}, err
	}
	_, _, outputName, err := parseXrandrOutput(raw)
	if err != nil {
		return Resolution{}, err
	}

	if freq == 0 {
		if resolved, ferr := pickFreq(width, height); ferr == nil {
			freq = resolved
		}
	}

	args := []string{"--output", outputName, "--mode", fmt.Sprintf("%dx%d", width, height)}
	if freq != 0 {
		args = append(args, "--rate", strconv.FormatUint(uint64(freq), 10))
	}

	if out, xerr := exec.Command("xrandr", args...).CombinedOutput(); xerr != nil {
		return Resolution{}, fmt.Errorf("xrandr failed: %s", strings.TrimSpace(string(out)))
	}

	cur, err := GetCurrent()
	if err != nil {
		return Resolution{Width: width, Height: height, Freq: freq}, nil
	}
	return cur, nil
}

// pickFreq finds the best refresh rate for width×height.
// Prefers the current rate; falls back to the highest available.
func pickFreq(width, height uint32) (uint32, error) {
	modes, err := ListModes()
	if err != nil {
		return 0, err
	}

	cur, _ := GetCurrent()

	var best uint32
	var found bool
	for _, m := range modes {
		if m.Width != width || m.Height != height {
			continue
		}
		if !found || m.Freq > best {
			best = m.Freq
			found = true
		}
		if m.Freq == cur.Freq {
			return m.Freq, nil
		}
	}

	if !found {
		return 0, fmt.Errorf("resolution %dx%d is not supported by this display", width, height)
	}
	return best, nil
}

