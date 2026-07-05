//go:build linux

package main

import (
	"encoding/binary"
	"strings"
	"testing"
)

// --- compositorHint ---

func TestCompositorHint(t *testing.T) {
	tests := []struct {
		desktop string
		session string
		wantIn  string
	}{
		{"GNOME", "", "gnome-randr"},
		{"ubuntu:GNOME", "", "gnome-randr"},
		{"gnome", "", "gnome-randr"}, // lowercase
		{"KDE", "", "kscreen-doctor"},
		{"kde-plasma", "", "kscreen-doctor"},
		{"PLASMA", "", "kscreen-doctor"},
		{"", "plasma", "kscreen-doctor"}, // via DESKTOP_SESSION fallback
		{"sway", "", "wlroots"},
		{"Hyprland", "", "wlroots"},
		{"", "", "wlroots"}, // unknown → generic message
	}

	for _, tt := range tests {
		t.Run(tt.desktop+"/"+tt.session, func(t *testing.T) {
			t.Setenv("XDG_CURRENT_DESKTOP", tt.desktop)
			t.Setenv("DESKTOP_SESSION", tt.session)
			hint := compositorHint()
			if !strings.Contains(strings.ToLower(hint), strings.ToLower(tt.wantIn)) {
				t.Errorf("compositorHint() = %q, want it to contain %q", hint, tt.wantIn)
			}
		})
	}
}

// --- pickFreqFromHead ---

func TestPickFreqFromHead(t *testing.T) {
	mode60 := &wlrModeInfo{id: 1, width: 1920, height: 1080, refreshHz: 60}
	mode120 := &wlrModeInfo{id: 2, width: 1920, height: 1080, refreshHz: 120}
	mode144 := &wlrModeInfo{id: 3, width: 1920, height: 1080, refreshHz: 144}
	mode4k60 := &wlrModeInfo{id: 4, width: 3840, height: 2160, refreshHz: 60}
	modeFinished := &wlrModeInfo{id: 5, width: 1920, height: 1080, refreshHz: 75, finished: true}

	c := &wlrClient{
		modes: map[uint32]*wlrModeInfo{
			1: mode60, 2: mode120, 3: mode144, 4: mode4k60, 5: modeFinished,
		},
	}
	head := &wlrHeadInfo{
		modes: []*wlrModeInfo{mode60, mode120, mode144, mode4k60, modeFinished},
	}

	t.Run("prefers current freq", func(t *testing.T) {
		head.currentMode = 2 // 120Hz active
		if got := c.pickFreqFromHead(head, 1920, 1080); got != 120 {
			t.Errorf("got %d, want 120", got)
		}
	})

	t.Run("falls back to highest when current not available at new res", func(t *testing.T) {
		head.currentMode = 3 // 144Hz — not available at 4K
		if got := c.pickFreqFromHead(head, 3840, 2160); got != 60 {
			t.Errorf("got %d, want 60", got)
		}
	})

	t.Run("ignores finished current mode, returns highest", func(t *testing.T) {
		head.currentMode = 5 // finished 75Hz mode
		if got := c.pickFreqFromHead(head, 1920, 1080); got != 144 {
			t.Errorf("got %d, want 144 (finished current mode must be ignored)", got)
		}
	})

	t.Run("returns 0 when no mode matches resolution", func(t *testing.T) {
		head.currentMode = 1
		if got := c.pickFreqFromHead(head, 2560, 1440); got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})
}

// --- findMode ---

func TestFindMode(t *testing.T) {
	mode60 := &wlrModeInfo{id: 1, width: 1920, height: 1080, refreshHz: 60}
	mode144 := &wlrModeInfo{id: 2, width: 1920, height: 1080, refreshHz: 144}
	modeFinished := &wlrModeInfo{id: 3, width: 1920, height: 1080, refreshHz: 120, finished: true}

	head := &wlrHeadInfo{
		modes: []*wlrModeInfo{mode60, mode144, modeFinished},
	}
	c := &wlrClient{}

	t.Run("no freq constraint picks highest", func(t *testing.T) {
		if got := c.findMode(head, 1920, 1080, 0); got != mode144 {
			t.Errorf("got %v, want mode144", got)
		}
	})

	t.Run("exact freq match", func(t *testing.T) {
		if got := c.findMode(head, 1920, 1080, 60); got != mode60 {
			t.Errorf("got %v, want mode60", got)
		}
	})

	t.Run("finished mode is never returned", func(t *testing.T) {
		if got := c.findMode(head, 1920, 1080, 120); got != nil {
			t.Errorf("got %v, want nil (finished mode)", got)
		}
	})

	t.Run("unknown resolution returns nil", func(t *testing.T) {
		if got := c.findMode(head, 2560, 1440, 0); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestHeadByName(t *testing.T) {
	headA := &wlrHeadInfo{id: 1, name: "DP-1", enabled: true}
	headB := &wlrHeadInfo{id: 2, name: "HDMI-A-1", enabled: false}
	headFinished := &wlrHeadInfo{id: 3, name: "DP-2", enabled: true, finished: true}

	c := &wlrClient{
		heads: map[uint32]*wlrHeadInfo{
			1: headA,
			2: headB,
			3: headFinished,
		},
		headOrder: []uint32{1, 2, 3},
	}

	if got := c.headByName("DP-1"); got != headA {
		t.Fatalf("headByName(DP-1) = %v, want %v", got, headA)
	}
	if got := c.headByName("HDMI-A-1"); got != nil {
		t.Fatalf("headByName(disabled) = %v, want nil", got)
	}
	if got := c.headByName("DP-2"); got != nil {
		t.Fatalf("headByName(finished) = %v, want nil", got)
	}
	if got := c.headByName("missing"); got != nil {
		t.Fatalf("headByName(missing) = %v, want nil", got)
	}
}

// --- onHeadEvent opcodes 6/7/8 ---

func makeU32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func makeI32x2(x, y int32) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], uint32(x))
	binary.LittleEndian.PutUint32(b[4:8], uint32(y))
	return b
}

func TestOnHeadEvent_Position(t *testing.T) {
	c := &wlrClient{}
	head := &wlrHeadInfo{}

	c.onHeadEvent(head, 6, makeI32x2(1920, 0))
	if head.posX != 1920 || head.posY != 0 {
		t.Errorf("position = (%d, %d), want (1920, 0)", head.posX, head.posY)
	}

	c.onHeadEvent(head, 6, makeI32x2(-100, 200))
	if head.posX != -100 || head.posY != 200 {
		t.Errorf("position = (%d, %d), want (-100, 200)", head.posX, head.posY)
	}
}

func TestOnHeadEvent_Transform(t *testing.T) {
	c := &wlrClient{}
	head := &wlrHeadInfo{}

	for _, tr := range []uint32{0, 1, 2, 3, 4} {
		c.onHeadEvent(head, 7, makeU32(tr))
		if head.transform != tr {
			t.Errorf("transform = %d, want %d", head.transform, tr)
		}
	}
}

func TestOnHeadEvent_Scale(t *testing.T) {
	c := &wlrClient{}
	head := &wlrHeadInfo{}

	// 1.0 = 256, 1.5 = 384, 2.0 = 512 in wl_fixed 24.8
	for _, scale := range []int32{256, 384, 512} {
		c.onHeadEvent(head, 8, makeU32(uint32(scale)))
		if head.scale != scale {
			t.Errorf("scale = %d, want %d", head.scale, scale)
		}
	}
}

func TestOnHeadEvent_ShortDataIgnored(t *testing.T) {
	c := &wlrClient{}
	head := &wlrHeadInfo{posX: 10, posY: 20, transform: 1, scale: 256}

	// Sending too-short payloads must not mutate state
	c.onHeadEvent(head, 6, []byte{1, 2, 3}) // needs 8 bytes
	if head.posX != 10 || head.posY != 20 {
		t.Error("short position payload must not modify posX/posY")
	}
	c.onHeadEvent(head, 7, []byte{1, 2}) // needs 4 bytes
	if head.transform != 1 {
		t.Error("short transform payload must not modify transform")
	}
	c.onHeadEvent(head, 8, []byte{1}) // needs 4 bytes
	if head.scale != 256 {
		t.Error("short scale payload must not modify scale")
	}
}
