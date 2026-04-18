//go:build linux

package main

import (
	"testing"
)

func TestParseWlrRandrOutput(t *testing.T) {
	const sample = `HDMI-A-1 "Dell U2720Q" (0x12345678)
  Physical size: 600x340 mm
  Enabled: yes
  Modes:
    3840x2160 px, 60.000000 Hz (preferred, current)
    3840x2160 px, 30.000000 Hz
    1920x1080 px, 60.000000 Hz
  Position: 0,0
  Transform: normal
  Scale: 1.000000
`

	modes, current, outputName, err := parseWlrRandrOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputName != "HDMI-A-1" {
		t.Errorf("outputName = %q, want %q", outputName, "HDMI-A-1")
	}
	if current != (Resolution{Width: 3840, Height: 2160, Freq: 60}) {
		t.Errorf("current = %v, want 3840x2160@60", current)
	}
	if len(modes) != 3 {
		t.Errorf("len(modes) = %d, want 3", len(modes))
	}
	// Modes should be sorted by width → height → freq.
	want := []Resolution{
		{1920, 1080, 60},
		{3840, 2160, 30},
		{3840, 2160, 60},
	}
	for i, m := range modes {
		if m != want[i] {
			t.Errorf("modes[%d] = %v, want %v", i, m, want[i])
		}
	}
}

func TestParseWlrRandrOutput_DisabledOutputSkipped(t *testing.T) {
	const sample = `DP-1 "Some Monitor" (0xABCDEF01)
  Physical size: 527x296 mm
  Enabled: no
  Modes:
    1920x1080 px, 60.000000 Hz (preferred)
  Position: 0,0
HDMI-A-1 "Active Monitor" (0x12345678)
  Physical size: 527x296 mm
  Enabled: yes
  Modes:
    1920x1080 px, 60.000000 Hz (preferred, current)
  Position: 1920,0
`

	_, _, outputName, err := parseWlrRandrOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputName != "HDMI-A-1" {
		t.Errorf("outputName = %q, want %q (disabled output should be skipped)", outputName, "HDMI-A-1")
	}
}

func TestParseWlrRandrOutput_NoEnabledOutput(t *testing.T) {
	const sample = `DP-1 "Monitor" (0x...)
  Enabled: no
  Modes:
    1920x1080 px, 60.000000 Hz
`
	_, _, _, err := parseWlrRandrOutput(sample)
	if err == nil {
		t.Error("expected error for no enabled output, got nil")
	}
}

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
