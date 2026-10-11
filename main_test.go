package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseResolution(t *testing.T) {
	tests := []struct {
		input   string
		width   uint32
		height  uint32
		freq    uint32
		wantErr bool
	}{
		{"1920x1080", 1920, 1080, 0, false},
		{"2560x1440@144", 2560, 1440, 144, false},
		{"2560x1440@144hz", 2560, 1440, 144, false},
		{"2560x1440@144Hz", 2560, 1440, 144, false},
		{"  1920x1080 ", 1920, 1080, 0, false},
		{"3840x2160@60", 3840, 2160, 60, false},
		{"badformat", 0, 0, 0, true},
		{"axb", 0, 0, 0, true},
		{"1920x1080@notanumber", 0, 0, 0, true},
		{"x1080", 0, 0, 0, true},
		{"1920x", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			w, h, f, err := parseResolution(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseResolution(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseResolution(%q): unexpected error: %v", tt.input, err)
			}
			if w != tt.width || h != tt.height || f != tt.freq {
				t.Errorf("parseResolution(%q) = (%d, %d, %d), want (%d, %d, %d)",
					tt.input, w, h, f, tt.width, tt.height, tt.freq)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("buf.ReadFrom: %v", err)
	}
	return buf.String()
}

func TestPrintJSON_Resolution(t *testing.T) {
	res := Resolution{Width: 1920, Height: 1080, Freq: 60}
	out := captureStdout(t, func() {
		if err := printJSON(res); err != nil {
			t.Fatalf("printJSON: %v", err)
		}
	})

	var got Resolution
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v — output was: %s", err, out)
	}
	if got != res {
		t.Errorf("got %+v, want %+v", got, res)
	}
}

func TestPrintJSON_ResolutionSlice(t *testing.T) {
	modes := []Resolution{
		{Width: 1920, Height: 1080, Freq: 60},
		{Width: 2560, Height: 1440, Freq: 144},
	}
	out := captureStdout(t, func() {
		if err := printJSON(modes); err != nil {
			t.Fatalf("printJSON: %v", err)
		}
	})

	var got []Resolution
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v — output was: %s", err, out)
	}
	if len(got) != len(modes) {
		t.Fatalf("got %d items, want %d", len(got), len(modes))
	}
	for i, m := range modes {
		if got[i] != m {
			t.Errorf("item %d: got %+v, want %+v", i, got[i], m)
		}
	}
}

func TestPrintJSON_Keys(t *testing.T) {
	out := captureStdout(t, func() {
		_ = printJSON(Resolution{Width: 1920, Height: 1080, Freq: 60})
	})

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"width", "height", "freq"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing key %q: %s", key, out)
		}
	}
}

func TestParseCommandFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantJSON    bool
		wantDisplay string
		wantForce   bool
		wantList    bool
		wantClear   bool
		wantErr     bool
	}{
		{
			name:        "display separate token",
			args:        []string{"--json", "--display", "DP-1", "1920x1080"},
			wantArgs:    []string{"1920x1080"},
			wantJSON:    true,
			wantDisplay: "DP-1",
		},
		{
			name:        "display equals syntax",
			args:        []string{"toggle", "--display=HDMI-1", "1920x1080"},
			wantArgs:    []string{"toggle", "1920x1080"},
			wantDisplay: "HDMI-1",
		},
		{
			name:      "force flag",
			args:      []string{"1920x1080", "--force"},
			wantArgs:  []string{"1920x1080"},
			wantForce: true,
		},
		{
			name:     "list flag",
			args:     []string{"--list"},
			wantArgs: []string{},
			wantList: true,
		},
		{
			name:      "clear flag",
			args:      []string{"--clear"},
			wantArgs:  []string{},
			wantClear: true,
		},
		{
			name:    "list and clear together is an error",
			args:    []string{"--list", "--clear"},
			wantErr: true,
		},
		{
			name:    "missing display value",
			args:    []string{"--display"},
			wantErr: true,
		},
		{
			name:    "empty display value",
			args:    []string{"--display="},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotFlags, err := parseCommandFlags(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCommandFlags(%v): %v", tt.args, err)
			}
			if gotFlags.JSON != tt.wantJSON {
				t.Errorf("JSON = %v, want %v", gotFlags.JSON, tt.wantJSON)
			}
			if gotFlags.Display != tt.wantDisplay {
				t.Errorf("Display = %q, want %q", gotFlags.Display, tt.wantDisplay)
			}
			if gotFlags.Force != tt.wantForce {
				t.Errorf("Force = %v, want %v", gotFlags.Force, tt.wantForce)
			}
			if gotFlags.List != tt.wantList {
				t.Errorf("List = %v, want %v", gotFlags.List, tt.wantList)
			}
			if gotFlags.Clear != tt.wantClear {
				t.Errorf("Clear = %v, want %v", gotFlags.Clear, tt.wantClear)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("args len = %d, want %d (%v)", len(gotArgs), len(tt.wantArgs), gotArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Fatalf("args[%d] = %q, want %q", i, gotArgs[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestIsNoOp(t *testing.T) {
	cur := Resolution{Width: 1920, Height: 1080, Freq: 60}

	tests := []struct {
		name   string
		width  uint32
		height uint32
		freq   uint32
		want   bool
	}{
		{"same resolution, freq unspecified", 1920, 1080, 0, true},
		{"same resolution and freq", 1920, 1080, 60, true},
		{"same resolution, different freq", 1920, 1080, 144, false},
		{"different width", 2560, 1080, 0, false},
		{"different height", 1920, 1440, 0, false},
		{"different resolution and freq", 2560, 1440, 144, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNoOp(cur, tt.width, tt.height, tt.freq); got != tt.want {
				t.Errorf("isNoOp(%+v, %d, %d, %d) = %v, want %v", cur, tt.width, tt.height, tt.freq, got, tt.want)
			}
		})
	}
}

func TestGroupModesByResolution(t *testing.T) {
	modes := []Resolution{
		{Width: 1920, Height: 1080, Freq: 50},
		{Width: 1920, Height: 1080, Freq: 60},
		{Width: 2560, Height: 1440, Freq: 60},
		{Width: 2560, Height: 1440, Freq: 120},
		{Width: 2560, Height: 1440, Freq: 144},
	}

	want := []modeGroup{
		{Width: 1920, Height: 1080, Freqs: []uint32{50, 60}},
		{Width: 2560, Height: 1440, Freqs: []uint32{60, 120, 144}},
	}

	got := groupModesByResolution(modes)
	if len(got) != len(want) {
		t.Fatalf("groupModesByResolution() returned %d groups, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Width != want[i].Width || got[i].Height != want[i].Height {
			t.Fatalf("group[%d] = %dx%d, want %dx%d", i, got[i].Width, got[i].Height, want[i].Width, want[i].Height)
		}
		if len(got[i].Freqs) != len(want[i].Freqs) {
			t.Fatalf("group[%d].Freqs = %v, want %v", i, got[i].Freqs, want[i].Freqs)
		}
		for j := range want[i].Freqs {
			if got[i].Freqs[j] != want[i].Freqs[j] {
				t.Errorf("group[%d].Freqs[%d] = %d, want %d", i, j, got[i].Freqs[j], want[i].Freqs[j])
			}
		}
	}
}

func TestGroupModesByResolution_Empty(t *testing.T) {
	if got := groupModesByResolution(nil); len(got) != 0 {
		t.Errorf("groupModesByResolution(nil) = %+v, want empty", got)
	}
}

// withTempConfigDir redirects every platform's config-directory lookup
// (Linux's XDG_CONFIG_HOME, Darwin's $HOME, Windows' %APPDATA%) at a fresh
// temp dir, so loadState/saveState/clearState tests don't touch the real
// user config regardless of which OS runs them.
func withTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("APPDATA", dir)
}

func TestPrintToggleList_NoSavedState(t *testing.T) {
	withTempConfigDir(t)

	out := captureStdout(t, func() {
		if err := printToggleList(""); err != nil {
			t.Fatalf("printToggleList: %v", err)
		}
	})

	if !strings.Contains(out, "No toggle list configured") {
		t.Errorf("output = %q, want a message about no toggle list configured", out)
	}
}

func TestPrintToggleList_WithSavedState(t *testing.T) {
	withTempConfigDir(t)

	state := ToggleState{Resolutions: []string{"1920x1080", "2560x1440"}, CurrentIndex: 1}
	if err := saveState(state, ""); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := printToggleList(""); err != nil {
			t.Fatalf("printToggleList: %v", err)
		}
	})

	wantLines := []string{
		"Toggle list:",
		"  [0] 1920x1080",
		"  [1] 2560x1440  ← current",
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	}
}

func TestClearToggleList_NoSavedState(t *testing.T) {
	withTempConfigDir(t)

	out := captureStdout(t, func() {
		if err := clearToggleList(""); err != nil {
			t.Fatalf("clearToggleList: %v", err)
		}
	})

	if !strings.Contains(out, "No toggle list to clear") {
		t.Errorf("output = %q, want a message about no toggle list to clear", out)
	}
}

func TestClearToggleList_WithSavedState(t *testing.T) {
	withTempConfigDir(t)

	state := ToggleState{Resolutions: []string{"1920x1080"}}
	if err := saveState(state, ""); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := clearToggleList(""); err != nil {
			t.Fatalf("clearToggleList: %v", err)
		}
	})

	if !strings.Contains(out, "Toggle list cleared") {
		t.Errorf("output = %q, want a message about the toggle list being cleared", out)
	}

	if _, err := loadState(""); err == nil {
		t.Error("loadState after clear: expected error, got nil")
	}
}

func TestPrintToggleList_CorruptState(t *testing.T) {
	withTempConfigDir(t)

	path := stateFile("")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := printToggleList("")
	if err == nil {
		t.Fatal("printToggleList with corrupt state: expected error, got nil")
	}
	if strings.Contains(err.Error(), "No toggle list configured") {
		t.Errorf("corrupt state should not be reported as \"no toggle list configured\": %v", err)
	}
}

func TestCmdToggle_ListOrClearWithResolutionArgs(t *testing.T) {
	withTempConfigDir(t)

	if err := cmdToggle([]string{"1920x1080"}, "", true, false); err == nil {
		t.Error("cmdToggle with --list and resolution args: expected error, got nil")
	}
	if err := cmdToggle([]string{"1920x1080"}, "", false, true); err == nil {
		t.Error("cmdToggle with --clear and resolution args: expected error, got nil")
	}
}
