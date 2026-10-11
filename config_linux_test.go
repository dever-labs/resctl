//go:build linux

package main

import (
	"os"
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

func TestSaveAndLoadState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	want := ToggleState{
		Resolutions:  []string{"1920x1080", "2560x1440@144"},
		CurrentIndex: 1,
	}

	if err := saveState(want, ""); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	got, err := loadState("")
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if got.CurrentIndex != want.CurrentIndex || len(got.Resolutions) != len(want.Resolutions) {
		t.Fatalf("loadState() = %+v, want %+v", got, want)
	}
	for i := range want.Resolutions {
		if got.Resolutions[i] != want.Resolutions[i] {
			t.Errorf("Resolutions[%d] = %q, want %q", i, got.Resolutions[i], want.Resolutions[i])
		}
	}
}

func TestSaveAndLoadState_PerDisplay(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	dp1 := ToggleState{Resolutions: []string{"1920x1080"}, CurrentIndex: 0}
	dp2 := ToggleState{Resolutions: []string{"2560x1440"}, CurrentIndex: 0}

	if err := saveState(dp1, "DP-1"); err != nil {
		t.Fatalf("saveState(DP-1): %v", err)
	}
	if err := saveState(dp2, "DP-2"); err != nil {
		t.Fatalf("saveState(DP-2): %v", err)
	}

	got1, err := loadState("DP-1")
	if err != nil {
		t.Fatalf("loadState(DP-1): %v", err)
	}
	if len(got1.Resolutions) != 1 || got1.Resolutions[0] != "1920x1080" {
		t.Fatalf("loadState(DP-1) = %+v, want %+v", got1, dp1)
	}

	got2, err := loadState("DP-2")
	if err != nil {
		t.Fatalf("loadState(DP-2): %v", err)
	}
	if len(got2.Resolutions) != 1 || got2.Resolutions[0] != "2560x1440" {
		t.Fatalf("loadState(DP-2) = %+v, want %+v", got2, dp2)
	}
}

func TestLoadState_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, err := loadState(""); err == nil {
		t.Fatal("loadState() on missing file: expected error, got nil")
	}
}

func TestLoadState_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := stateFile("")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := loadState(""); err == nil {
		t.Fatal("loadState() on corrupt file: expected error, got nil")
	}
}

func TestSaveState_CreatesConfigDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)

	if err := saveState(ToggleState{Resolutions: []string{"1920x1080"}}, ""); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	info, err := os.Stat(filepath.Join(base, "resctl"))
	if err != nil {
		t.Fatalf("expected config dir to be created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected resctl config path to be a directory")
	}
}
