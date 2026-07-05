//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ToggleState persists the user's toggle list and current position.
type ToggleState struct {
	Resolutions  []string `json:"resolutions"`
	CurrentIndex int      `json:"currentIndex"`
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "resctl")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "resctl")
}

func sanitizeDisplayName(display string) string {
	var b strings.Builder
	for _, r := range display {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

func stateFile(display string) string {
	if display == "" {
		return filepath.Join(configDir(), "state.json")
	}
	return filepath.Join(configDir(), "state-"+sanitizeDisplayName(display)+".json")
}

func loadState(display string) (ToggleState, error) {
	data, err := os.ReadFile(stateFile(display))
	if err != nil {
		return ToggleState{}, err
	}
	var state ToggleState
	if err := json.Unmarshal(data, &state); err != nil {
		return ToggleState{}, err
	}
	return state, nil
}

func saveState(state ToggleState, display string) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile(display), data, 0o644)
}
