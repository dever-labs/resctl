//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>

static CGDisplayModeRef resctl_current_mode(CGDirectDisplayID display) {
	return CGDisplayCopyDisplayMode(display);
}

static CFArrayRef resctl_all_modes(CGDirectDisplayID display) {
	const void *keys[] = { kCGDisplayShowDuplicateLowResolutionModes };
	const void *values[] = { kCFBooleanTrue };
	CFDictionaryRef options = CFDictionaryCreate(
		kCFAllocatorDefault,
		keys,
		values,
		1,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	CFArrayRef modes = CGDisplayCopyAllDisplayModes(display, options);
	if (options != NULL) {
		CFRelease(options);
	}
	return modes;
}

static uint32_t resctl_active_displays(CGDirectDisplayID *displays, uint32_t max) {
	uint32_t count = 0;
	CGError err = CGGetActiveDisplayList(max, displays, &count);
	if (err != kCGErrorSuccess) {
		return 0;
	}
	return count;
}

static CGDisplayModeRef resctl_mode_at(CFArrayRef modes, CFIndex index) {
	return (CGDisplayModeRef)CFArrayGetValueAtIndex(modes, index);
}

static void resctl_release_mode(CGDisplayModeRef mode) {
	if (mode != NULL) {
		CFRelease(mode);
	}
}

static void resctl_release_modes(CFArrayRef modes) {
	if (modes != NULL) {
		CFRelease(modes);
	}
}
*/
import "C"

import (
	"fmt"
	"sort"
	"strconv"
)

// Resolution holds a display mode.
type Resolution struct {
	Width  uint32 `json:"width"`
	Height uint32 `json:"height"`
	Freq   uint32 `json:"freq"`
}

func (r Resolution) String() string {
	return fmt.Sprintf("%dx%d@%dHz", r.Width, r.Height, r.Freq)
}

func resolveDisplayID(display string) (C.CGDirectDisplayID, error) {
	if display == "" {
		return C.CGMainDisplayID(), nil
	}

	index, err := strconv.Atoi(display)
	if err != nil || index < 1 {
		return 0, fmt.Errorf("display %q must be a 1-based index like 1 or 2", display)
	}

	var displays [32]C.CGDirectDisplayID
	count := int(C.resctl_active_displays(&displays[0], C.uint32_t(len(displays))))
	if count == 0 {
		return 0, fmt.Errorf("could not enumerate active displays")
	}
	if index > count {
		return 0, fmt.Errorf("display %q is out of range (found %d active displays)", display, count)
	}
	return displays[index-1], nil
}

func resolutionFromMode(mode C.CGDisplayModeRef) Resolution {
	refresh := C.CGDisplayModeGetRefreshRate(mode)
	var freq uint32
	if refresh > 0 {
		freq = uint32(refresh + 0.5)
	}

	return Resolution{
		Width:  uint32(C.CGDisplayModeGetWidth(mode)),
		Height: uint32(C.CGDisplayModeGetHeight(mode)),
		Freq:   freq,
	}
}

// GetCurrent returns the selected display's current resolution.
func GetCurrent(display string) (Resolution, error) {
	displayID, err := resolveDisplayID(display)
	if err != nil {
		return Resolution{}, err
	}

	mode := C.resctl_current_mode(displayID)
	if mode == 0 {
		return Resolution{}, fmt.Errorf("CGDisplayCopyDisplayMode failed")
	}
	defer C.resctl_release_mode(mode)

	return resolutionFromMode(mode), nil
}

// ListModes returns all unique display modes for the selected display.
func ListModes(display string) ([]Resolution, error) {
	displayID, err := resolveDisplayID(display)
	if err != nil {
		return nil, err
	}

	modesRef := C.resctl_all_modes(displayID)
	if modesRef == 0 {
		return nil, fmt.Errorf("CGDisplayCopyAllDisplayModes failed")
	}
	defer C.resctl_release_modes(modesRef)

	seen := make(map[string]bool)
	var modes []Resolution
	count := C.CFArrayGetCount(modesRef)
	for i := C.CFIndex(0); i < count; i++ {
		mode := C.resctl_mode_at(modesRef, i)
		if mode == 0 {
			continue
		}
		res := resolutionFromMode(mode)
		key := fmt.Sprintf("%dx%d@%d", res.Width, res.Height, res.Freq)
		if !seen[key] {
			seen[key] = true
			modes = append(modes, res)
		}
	}

	sort.Slice(modes, func(i, j int) bool {
		a, b := modes[i], modes[j]
		if a.Width != b.Width {
			return a.Width < b.Width
		}
		if a.Height != b.Height {
			return a.Height < b.Height
		}
		return a.Freq < b.Freq
	})

	return modes, nil
}

// SetResolution changes the selected display resolution.
func SetResolution(width, height, freq uint32, display string) (Resolution, error) {
	displayID, err := resolveDisplayID(display)
	if err != nil {
		return Resolution{}, err
	}

	modesRef := C.resctl_all_modes(displayID)
	if modesRef == 0 {
		return Resolution{}, fmt.Errorf("CGDisplayCopyAllDisplayModes failed")
	}
	defer C.resctl_release_modes(modesRef)

	if freq == 0 {
		if resolved, err := pickFreq(width, height, display); err == nil {
			freq = resolved
		}
	}

	var target C.CGDisplayModeRef
	count := C.CFArrayGetCount(modesRef)
	for i := C.CFIndex(0); i < count; i++ {
		mode := C.resctl_mode_at(modesRef, i)
		if mode == 0 {
			continue
		}
		res := resolutionFromMode(mode)
		if res.Width == width && res.Height == height && (freq == 0 || res.Freq == freq) {
			target = mode
			break
		}
	}
	if target == 0 {
		if freq != 0 {
			return Resolution{}, fmt.Errorf("resolution %dx%d@%dHz is not supported", width, height, freq)
		}
		return Resolution{}, fmt.Errorf("resolution %dx%d is not supported", width, height)
	}

	if result := C.CGDisplaySetDisplayMode(displayID, target, C.CFDictionaryRef(0)); result != C.kCGErrorSuccess {
		return Resolution{}, fmt.Errorf("CGDisplaySetDisplayMode failed (code %d)", int32(result))
	}

	cur, err := GetCurrent(display)
	if err != nil {
		return Resolution{Width: width, Height: height, Freq: freq}, nil
	}
	return cur, nil
}

// pickFreq finds the best refresh rate for width x height.
// Prefers the current rate; falls back to the highest available.
func pickFreq(width, height uint32, display string) (uint32, error) {
	modes, err := ListModes(display)
	if err != nil {
		return 0, err
	}

	cur, _ := GetCurrent(display)

	var best uint32
	var found bool
	for _, m := range modes {
		if m.Width != width || m.Height != height {
			continue
		}
		if !found || m.Freq > best {
			best = m.Freq
			found = true
		}
		if m.Freq == cur.Freq {
			return m.Freq, nil
		}
	}

	if !found {
		return 0, fmt.Errorf("resolution %dx%d is not supported by this display", width, height)
	}
	return best, nil
}
