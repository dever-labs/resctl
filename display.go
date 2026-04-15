//go:build windows

package main

import (
	"fmt"
	"sort"
	"syscall"
	"unsafe"
)

const (
	CCHDEVICENAME = 32
	CCHFORMNAME   = 32

	CDS_UPDATEREGISTRY = 0x00000001

	DISP_CHANGE_SUCCESSFUL = 0
	DISP_CHANGE_RESTART    = 1
	DISP_CHANGE_FAILED     = -1
	DISP_CHANGE_BADMODE    = -2
	DISP_CHANGE_NOTUPDATED = -3
	DISP_CHANGE_BADFLAGS   = -4
	DISP_CHANGE_BADPARAM   = -5

	DM_BITSPERPEL       = 0x00040000
	DM_PELSWIDTH        = 0x00080000
	DM_PELSHEIGHT       = 0x00100000
	DM_DISPLAYFREQUENCY = 0x00400000

	ENUM_CURRENT_SETTINGS = 0xFFFFFFFF
)

// DEVMODE mirrors the Win32 DEVMODEW structure (220 bytes).
// The 16-byte union at offset 76 is represented by the eight print fields
// (dmOrientation … dmPrintQuality); display-position fields overlay them
// but we never need to read them directly.
type DEVMODE struct {
	DmDeviceName    [CCHDEVICENAME]uint16
	DmSpecVersion   uint16
	DmDriverVersion uint16
	DmSize          uint16
	DmDriverExtra   uint16
	DmFields        uint32
	// union: print fields (8 × int16 = 16 bytes)
	DmOrientation   int16
	DmPaperSize     int16
	DmPaperLength   int16
	DmPaperWidth    int16
	DmScale         int16
	DmCopies        int16
	DmDefaultSource int16
	DmPrintQuality  int16
	// end of union
	DmColor            int16
	DmDuplex           int16
	DmYResolution      int16
	DmTTOption         int16
	DmCollate          int16
	DmFormName         [CCHFORMNAME]uint16
	DmLogPixels        uint16
	DmBitsPerPel       uint32
	DmPelsWidth        uint32
	DmPelsHeight       uint32
	DmDisplayFlags     uint32
	DmDisplayFrequency uint32
	DmICMMethod        uint32
	DmICMIntent        uint32
	DmMediaType        uint32
	DmDitherType       uint32
	DmReserved1        uint32
	DmReserved2        uint32
	DmPanningWidth     uint32
	DmPanningHeight    uint32
}

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	procEnumDisplaySettingsW   = user32.NewProc("EnumDisplaySettingsW")
	procChangeDisplaySettingsW = user32.NewProc("ChangeDisplaySettingsW")
)

// Resolution holds a display mode.
type Resolution struct {
	Width  uint32
	Height uint32
	Freq   uint32
}

func (r Resolution) String() string {
	return fmt.Sprintf("%dx%d@%dHz", r.Width, r.Height, r.Freq)
}

// GetCurrent returns the primary display's current resolution.
func GetCurrent() (Resolution, error) {
	var dm DEVMODE
	dm.DmSize = uint16(unsafe.Sizeof(dm))
	ret, _, _ := procEnumDisplaySettingsW.Call(
		0,
		uintptr(ENUM_CURRENT_SETTINGS),
		uintptr(unsafe.Pointer(&dm)),
	)
	if ret == 0 {
		return Resolution{}, fmt.Errorf("EnumDisplaySettingsW failed")
	}
	return Resolution{
		Width:  dm.DmPelsWidth,
		Height: dm.DmPelsHeight,
		Freq:   dm.DmDisplayFrequency,
	}, nil
}

// ListModes returns all unique display modes for the primary monitor
// with at least 24-bit colour depth, sorted by width → height → freq.
func ListModes() ([]Resolution, error) {
	seen := make(map[string]bool)
	var modes []Resolution

	for i := uint32(0); ; i++ {
		var dm DEVMODE
		dm.DmSize = uint16(unsafe.Sizeof(dm))
		ret, _, _ := procEnumDisplaySettingsW.Call(
			0,
			uintptr(i),
			uintptr(unsafe.Pointer(&dm)),
		)
		if ret == 0 {
			break
		}
		if dm.DmBitsPerPel < 24 {
			continue
		}
		key := fmt.Sprintf("%dx%d@%d", dm.DmPelsWidth, dm.DmPelsHeight, dm.DmDisplayFrequency)
		if !seen[key] {
			seen[key] = true
			modes = append(modes, Resolution{
				Width:  dm.DmPelsWidth,
				Height: dm.DmPelsHeight,
				Freq:   dm.DmDisplayFrequency,
			})
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

// SetResolution changes the primary display resolution.
// When freq is 0, the current refresh rate is preserved if available at the
// new dimensions; otherwise the highest supported rate is used.
func SetResolution(width, height, freq uint32) (Resolution, error) {
	if freq == 0 {
		resolved, err := pickFreq(width, height)
		if err != nil {
			return Resolution{}, err
		}
		freq = resolved
	}

	var dm DEVMODE
	dm.DmSize = uint16(unsafe.Sizeof(dm))
	dm.DmFields = DM_PELSWIDTH | DM_PELSHEIGHT | DM_DISPLAYFREQUENCY
	dm.DmPelsWidth = width
	dm.DmPelsHeight = height
	dm.DmDisplayFrequency = freq

	ret, _, _ := procChangeDisplaySettingsW.Call(
		uintptr(unsafe.Pointer(&dm)),
		uintptr(CDS_UPDATEREGISTRY),
	)

	switch int32(ret) {
	case DISP_CHANGE_SUCCESSFUL:
		return Resolution{Width: width, Height: height, Freq: freq}, nil
	case DISP_CHANGE_RESTART:
		return Resolution{Width: width, Height: height, Freq: freq},
			fmt.Errorf("resolution changed but a restart is required")
	case DISP_CHANGE_BADMODE:
		return Resolution{}, fmt.Errorf("resolution %dx%d@%dHz is not supported", width, height, freq)
	default:
		return Resolution{}, fmt.Errorf("ChangeDisplaySettingsW failed (code %d)", int32(ret))
	}
}

// pickFreq finds the best refresh rate for width×height.
// Prefers the current rate; falls back to the highest available.
func pickFreq(width, height uint32) (uint32, error) {
	modes, err := ListModes()
	if err != nil {
		return 0, err
	}

	cur, _ := GetCurrent()

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
			// exact match with current rate – stop looking
			return m.Freq, nil
		}
	}

	if !found {
		return 0, fmt.Errorf("resolution %dx%d is not supported by this display", width, height)
	}
	return best, nil
}
