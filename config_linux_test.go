//go:build linux

package main

import (
	"path/filepath"
	"testing"
)

func TestStateFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/home/test/.config")

	if got := stateFile(""); got != filepath.Join("/home/test/.config", "resctl", "state.json") {
		t.Fatalf("stateFile(\"\") = %q", got)
	}

	if got := stateFile(`DP/1 Office\Monitor`); got != filepath.Join("/home/test/.config", "resctl", "state-DP-1-Office-Monitor.json") {
		t.Fatalf("stateFile(display) = %q", got)
	}
}

func TestSanitizeDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"DP-1", "DP-1"},
		{"HDMI-A-1", "HDMI-A-1"},
		{`DP/1`, "DP-1"},
		{`Office\Monitor`, "Office-Monitor"},
		{"My Monitor", "My-Monitor"},
		{`\\.\DISPLAY1`, "--.-DISPLAY1"},
		{"4K:Monitor*?", "4K-Monitor--"},
		{"monitor<1>", "monitor-1-"},
		{`a"b`, "a-b"},
		{"display_1.0", "display_1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeDisplayName(tt.input); got != tt.want {
				t.Errorf("sanitizeDisplayName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
