//go:build linux

package main

import (
	"testing"
)

func TestParseXrandrOutput(t *testing.T) {
	const sample = `Screen 0: minimum 8 x 8, current 1920 x 1080, maximum 32767 x 32767
HDMI-1 connected primary 1920x1080+0+0 (normal left inverted right x axis y axis) 527mm x 296mm
   1920x1080     60.00*+  50.00
   1280x720      60.00
DP-1 disconnected (normal left inverted right x axis y axis)
`

	modes, current, outputName, err := parseXrandrOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputName != "HDMI-1" {
		t.Errorf("outputName = %q, want %q", outputName, "HDMI-1")
	}
	if current != (Resolution{Width: 1920, Height: 1080, Freq: 60}) {
		t.Errorf("current = %v, want 1920x1080@60", current)
	}
	if len(modes) != 3 {
		t.Errorf("len(modes) = %d, want 3", len(modes))
	}
}

// TestParseXrandrOutput_NoPrimary verifies the fallback to the first connected
// output when no output is marked as "primary".
func TestParseXrandrOutput_NoPrimary(t *testing.T) {
	const sample = `Screen 0: minimum 8 x 8, current 2560 x 1440, maximum 32767 x 32767
DP-1 connected 2560x1440+0+0 (normal left inverted right x axis y axis) 597mm x 336mm
   2560x1440    144.00*+  60.00
HDMI-1 connected 1920x1080+2560+0 (normal left inverted right x axis y axis) 527mm x 296mm
   1920x1080     60.00*+
`

	_, _, outputName, err := parseXrandrOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without a "primary" marker, the first connected output (DP-1) is used.
	if outputName != "DP-1" {
		t.Errorf("outputName = %q, want %q (first connected fallback)", outputName, "DP-1")
	}
}

// TestParseXrandrOutput_PrimaryPreferredOverFirst verifies that when a later
// output is marked primary it takes precedence over the first connected one.
func TestParseXrandrOutput_PrimaryPreferredOverFirst(t *testing.T) {
	const sample = `Screen 0: minimum 8 x 8, current 1920 x 1080, maximum 32767 x 32767
DP-1 connected 1920x1080+0+0 (normal left inverted right x axis y axis) 527mm x 296mm
   1920x1080     60.00*+
HDMI-1 connected primary 2560x1440+1920+0 (normal left inverted right x axis y axis) 597mm x 336mm
   2560x1440    144.00*+  60.00
`

	_, _, outputName, err := parseXrandrOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputName != "HDMI-1" {
		t.Errorf("outputName = %q, want HDMI-1 (primary)", outputName)
	}
}

// TestParseXrandrOutput_NoConnected verifies that an error is returned when
// there are no connected outputs.
func TestParseXrandrOutput_NoConnected(t *testing.T) {
	const sample = `Screen 0: minimum 8 x 8, current 1920 x 1080, maximum 32767 x 32767
DP-1 disconnected (normal left inverted right x axis y axis)
HDMI-1 disconnected (normal left inverted right x axis y axis)
`

	_, _, _, err := parseXrandrOutput(sample)
	if err == nil {
		t.Error("expected error for no connected display, got nil")
	}
}

// TestParseXrandrOutput_HighRefreshRate verifies fractional Hz values are
// rounded correctly (e.g. 143.97 → 144).
func TestParseXrandrOutput_HighRefreshRate(t *testing.T) {
	const sample = `Screen 0: minimum 8 x 8, current 2560 x 1440, maximum 32767 x 32767
DP-1 connected primary 2560x1440+0+0 (normal left inverted right x axis y axis) 597mm x 336mm
   2560x1440    143.97*+  59.95
`

	modes, current, _, err := parseXrandrOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if current != (Resolution{Width: 2560, Height: 1440, Freq: 144}) {
		t.Errorf("current = %v, want 2560x1440@144 (143.97 rounds to 144)", current)
	}
	if len(modes) != 2 {
		t.Errorf("len(modes) = %d, want 2", len(modes))
	}
	if modes[0].Freq != 60 || modes[1].Freq != 144 {
		t.Errorf("modes = %v, want [{60} {144}]", modes)
	}
}

