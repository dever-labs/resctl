//go:build darwin

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ToggleState persists the user's toggle list and current position.
// Version identifies the schema so loadState() can migrate older files
// forward if the shape of this struct ever changes.
type ToggleState struct {
	Version      int      `json:"version"`
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
	// Files written before the version field existed have Version == 0;
	// treat them as v1, since the schema hasn't changed since then.
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func saveState(state ToggleState, display string) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	state.Version = currentStateVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(stateFile(display), data, 0o644)
}

// clearState deletes the saved toggle state file, if any. Returns an error
// satisfying os.IsNotExist if no state was saved.
func clearState(display string) error {
	return os.Remove(stateFile(display))
}
