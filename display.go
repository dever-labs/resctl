//go:build windows

package main

import (
	"fmt"
	"sort"
	"syscall"
	"unsafe"
)

const (
	CCHDEVICENAME   = 32
	CCHDEVICESTRING = 128
	CCHFORMNAME     = 32

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

type DISPLAY_DEVICE struct {
	Cb           uint32
	DeviceName   [CCHDEVICENAME]uint16
	DeviceString [CCHDEVICESTRING]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumDisplayDevicesW      = user32.NewProc("EnumDisplayDevicesW")
	procEnumDisplaySettingsW     = user32.NewProc("EnumDisplaySettingsW")
	procChangeDisplaySettingsW   = user32.NewProc("ChangeDisplaySettingsW")
	procChangeDisplaySettingsExW = user32.NewProc("ChangeDisplaySettingsExW")
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

func displayDeviceName(display string) (*uint16, error) {
	if display == "" {
		return nil, nil
	}

	for i := uint32(0); ; i++ {
		var dd DISPLAY_DEVICE
		dd.Cb = uint32(unsafe.Sizeof(dd))
		ret, _, _ := procEnumDisplayDevicesW.Call(
			0,
			uintptr(i),
			uintptr(unsafe.Pointer(&dd)),
			0,
		)
		if ret == 0 {
			break
		}

		deviceName := syscall.UTF16ToString(dd.DeviceName[:])
		deviceString := syscall.UTF16ToString(dd.DeviceString[:])
		if deviceName != display && deviceString != display {
			continue
		}

		ptr, err := syscall.UTF16PtrFromString(deviceName)
		if err != nil {
			return nil, err
		}
		return ptr, nil
	}

	return nil, fmt.Errorf("display %q not found", display)
}

// GetCurrent returns the selected display's current resolution.
func GetCurrent(display string) (Resolution, error) {
	deviceName, err := displayDeviceName(display)
	if err != nil {
		return Resolution{}, err
	}

	var dm DEVMODE
	dm.DmSize = uint16(unsafe.Sizeof(dm))
	ret, _, _ := procEnumDisplaySettingsW.Call(
		uintptr(unsafe.Pointer(deviceName)),
		uintptr(ENUM_CURRENT_SETTINGS),
		uintptr(unsafe.Pointer(&dm)),
	)
	if ret == 0 {
		if display == "" {
			return Resolution{}, fmt.Errorf("EnumDisplaySettingsW failed")
		}
		return Resolution{}, fmt.Errorf("EnumDisplaySettingsW failed for %q", display)
	}
	return Resolution{
		Width:  dm.DmPelsWidth,
		Height: dm.DmPelsHeight,
		Freq:   dm.DmDisplayFrequency,
	}, nil
}

// ListModes returns all unique display modes for the selected monitor
// with at least 24-bit colour depth, sorted by width → height → freq.
func ListModes(display string) ([]Resolution, error) {
	deviceName, err := displayDeviceName(display)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var modes []Resolution

	for i := uint32(0); ; i++ {
		var dm DEVMODE
		dm.DmSize = uint16(unsafe.Sizeof(dm))
		ret, _, _ := procEnumDisplaySettingsW.Call(
			uintptr(unsafe.Pointer(deviceName)),
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

// SetResolution changes the selected display resolution.
// When freq is 0, the current refresh rate is preserved if available at the
// new dimensions; otherwise the highest supported rate is used. If the
// resolution does not appear in the enumerated mode list (e.g. a virtual
// resolution injected by Sunshine), the frequency field is omitted entirely
// so Windows picks the best available rate rather than rejecting the call.
func SetResolution(width, height, freq uint32, display string) (Resolution, error) {
	deviceName, err := displayDeviceName(display)
	if err != nil {
		return Resolution{}, err
	}

	specifyFreq := freq != 0
	if !specifyFreq {
		if resolved, err := pickFreq(width, height, display); err == nil {
			freq = resolved
			specifyFreq = true
		}
		// If pickFreq fails the resolution is not in the enumerated mode list
		// (e.g. a virtual/injected resolution). Proceed without DM_DISPLAYFREQUENCY
		// and let Windows choose the refresh rate.
	}

	var dm DEVMODE
	dm.DmSize = uint16(unsafe.Sizeof(dm))
	dm.DmFields = DM_PELSWIDTH | DM_PELSHEIGHT
	dm.DmPelsWidth = width
	dm.DmPelsHeight = height
	if specifyFreq {
		dm.DmFields |= DM_DISPLAYFREQUENCY
		dm.DmDisplayFrequency = freq
	}

	var ret uintptr
	if deviceName == nil {
		ret, _, _ = procChangeDisplaySettingsW.Call(
			uintptr(unsafe.Pointer(&dm)),
			uintptr(CDS_UPDATEREGISTRY),
		)
	} else {
		ret, _, _ = procChangeDisplaySettingsExW.Call(
			uintptr(unsafe.Pointer(deviceName)),
			uintptr(unsafe.Pointer(&dm)),
			0,
			uintptr(CDS_UPDATEREGISTRY),
			0,
		)
	}

	switch int32(ret) {
	case DISP_CHANGE_SUCCESSFUL:
		if !specifyFreq {
			// Discover the actual refresh rate Windows applied.
			if cur, err := GetCurrent(display); err == nil {
				return cur, nil
			}
		}
		return Resolution{Width: width, Height: height, Freq: freq}, nil
	case DISP_CHANGE_RESTART:
		res := Resolution{Width: width, Height: height, Freq: freq}
		if !specifyFreq {
			if cur, err := GetCurrent(display); err == nil {
				res = cur
			}
		}
		return res, fmt.Errorf("resolution changed but a restart is required")
	case DISP_CHANGE_BADMODE:
		if specifyFreq {
			return Resolution{}, fmt.Errorf("resolution %dx%d@%dHz is not supported", width, height, freq)
		}
		return Resolution{}, fmt.Errorf("resolution %dx%d is not supported", width, height)
	default:
		apiName := "ChangeDisplaySettingsW"
		if display != "" {
			apiName = "ChangeDisplaySettingsExW"
		}
		return Resolution{}, fmt.Errorf("%s failed (code %d)", apiName, int32(ret))
	}
}

// pickFreq finds the best refresh rate for width×height.
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
			// exact match with current rate – stop looking
			return m.Freq, nil
		}
	}

	if !found {
		return 0, fmt.Errorf("resolution %dx%d is not supported by this display", width, height)
	}
	return best, nil
}
