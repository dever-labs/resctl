//go:build darwin

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
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "resctl")
	}
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "resctl")
}

func sanitizeDisplayName(display string) string {
	return strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(display)
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
