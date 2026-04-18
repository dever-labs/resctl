package main

import "testing"

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
