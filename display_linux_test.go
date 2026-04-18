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
