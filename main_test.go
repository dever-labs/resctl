package main

import (
	"bytes"
	"encoding/json"
	"os"
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
