//go:build windows

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// version is overridden at link time via -ldflags="-X main.version=vX.Y.Z".
var version = "dev"

const usage = `resctl - Windows 11 display resolution manager

Usage:
  resctl list                    List available resolutions
  resctl get                     Show current resolution
  resctl set <WxH[@Hz]>          Set resolution
  resctl toggle [res1 res2 ...]  Toggle between resolutions
  resctl install                 Copy to ~/bin and add to PATH
  resctl uninstall               Remove from ~/bin
  resctl version                 Print version

Resolution format:  WxH  or  WxH@Hz  (e.g. 1920x1080  or  2560x1440@144)

Examples:
  resctl set 1920x1080
  resctl set 2560x1440@144
  resctl toggle 1920x1080 2560x1440    # set list + switch immediately
  resctl toggle                        # cycle to next in saved list
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	cmd := strings.ToLower(os.Args[1])
	args := os.Args[2:]

	var err error
	switch cmd {
	case "list":
		err = cmdList()
	case "get":
		err = cmdGet()
	case "set":
		if len(args) == 0 {
			fatalf("set requires a resolution argument\n\n%s", usage)
		}
		err = cmdSet(args[0])
	case "toggle":
		err = cmdToggle(args)
	case "install":
		err = install()
	case "uninstall":
		err = uninstall()
	case "version", "-v", "--version":
		fmt.Printf("resctl %s\n", version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fatalf("unknown command %q\n\n%s", cmd, usage)
	}

	if err != nil {
		fatalf("%v\n", err)
	}
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format, a...)
	os.Exit(1)
}

func cmdList() error {
	modes, err := ListModes()
	if err != nil {
		return err
	}
	cur, _ := GetCurrent()

	fmt.Println("Available resolutions (primary display):")
	for _, m := range modes {
		active := "  "
		if m.Width == cur.Width && m.Height == cur.Height && m.Freq == cur.Freq {
			active = "* "
		}
		fmt.Printf("  %s%dx%d @ %dHz\n", active, m.Width, m.Height, m.Freq)
	}
	return nil
}

func cmdGet() error {
	cur, err := GetCurrent()
	if err != nil {
		return err
	}
	fmt.Printf("Current: %dx%d @ %dHz\n", cur.Width, cur.Height, cur.Freq)
	return nil
}

func cmdSet(arg string) error {
	width, height, freq, err := parseResolution(arg)
	if err != nil {
		return err
	}
	res, err := SetResolution(width, height, freq)
	if err != nil {
		return err
	}
	fmt.Printf("Set: %dx%d @ %dHz\n", res.Width, res.Height, res.Freq)
	return nil
}

func cmdToggle(args []string) error {
	var state ToggleState

	if len(args) > 0 {
		state.Resolutions = args
		// Position the index at the current resolution so the first toggle
		// moves to the next one in the list.
		cur, _ := GetCurrent()
		curBase := fmt.Sprintf("%dx%d", cur.Width, cur.Height)
		for i, r := range args {
			base := strings.SplitN(strings.ToLower(strings.TrimSpace(r)), "@", 2)[0]
			if base == curBase {
				state.CurrentIndex = i
				break
			}
		}
	} else {
		var err error
		state, err = loadState()
		if err != nil || len(state.Resolutions) == 0 {
			return fmt.Errorf("no toggle list configured\n" +
				"  Set one with: resctl toggle <res1> <res2> ...")
		}
	}

	state.CurrentIndex = (state.CurrentIndex + 1) % len(state.Resolutions)
	target := state.Resolutions[state.CurrentIndex]

	width, height, freq, err := parseResolution(target)
	if err != nil {
		return fmt.Errorf("invalid entry %q in toggle list: %w", target, err)
	}

	res, err := SetResolution(width, height, freq)
	if err != nil {
		return err
	}

	if saveErr := saveState(state); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save toggle state: %v\n", saveErr)
	}

	fmt.Printf("Toggled to: %dx%d @ %dHz\n", res.Width, res.Height, res.Freq)
	return nil
}

// parseResolution parses "WxH", "WxH@Hz", or "WxH@HzHz" into numeric parts.
// freq is 0 when not specified.
func parseResolution(s string) (width, height, freq uint32, err error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, "hz")

	var freqPart string
	if idx := strings.Index(s, "@"); idx != -1 {
		freqPart = strings.TrimSpace(s[idx+1:])
		s = s[:idx]
	}

	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0, 0, fmt.Errorf("invalid format %q (expected WxH or WxH@Hz)", s)
	}

	w, werr := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
	h, herr := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32)
	if werr != nil || herr != nil {
		return 0, 0, 0, fmt.Errorf("invalid resolution %q", s)
	}

	var f uint64
	if freqPart != "" {
		f, err = strconv.ParseUint(freqPart, 10, 32)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid frequency %q", freqPart)
		}
	}

	return uint32(w), uint32(h), uint32(f), nil
}
