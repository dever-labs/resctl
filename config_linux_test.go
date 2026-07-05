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
