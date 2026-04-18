//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func stateFile() string {
	return filepath.Join(configDir(), "state.json")
}

func loadState() (ToggleState, error) {
	data, err := os.ReadFile(stateFile())
	if err != nil {
		return ToggleState{}, err
	}
	var state ToggleState
	if err := json.Unmarshal(data, &state); err != nil {
		return ToggleState{}, err
	}
	return state, nil
}

func saveState(state ToggleState) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile(), data, 0o644)
}
