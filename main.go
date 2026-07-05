package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// version is overridden at link time via -ldflags="-X main.version=vX.Y.Z".
var version = "dev"

const usage = `resctl - display resolution manager

Usage:
  resctl list [--json] [--display <name>]           List available resolutions
  resctl get [--json] [--display <name>]            Show current resolution
  resctl set <WxH[@Hz]> [--display <name>]          Set resolution
  resctl toggle [res1 res2 ...] [--display <name>]  Toggle between resolutions
  resctl install                 Copy to ~/bin and add to PATH
  resctl uninstall               Remove from ~/bin
  resctl version                 Print version

Resolution format:  WxH  or  WxH@Hz  (e.g. 1920x1080  or  2560x1440@144)

Flags:
  --json              Output as JSON (supported by list and get)
  --display <name>    Target a specific display by name.
                      Linux: output name (e.g. DP-1, HDMI-1)
                      Windows: device name (e.g. \\.\DISPLAY1)
                      macOS: 1-based index (e.g. 1, 2)

Examples:
  resctl set 1920x1080
  resctl set 2560x1440@144
  resctl set 2560x1440@144 --display DP-1       # Linux
  resctl set 2560x1440@144 --display \\.\DISPLAY2  # Windows
  resctl set 2560x1440@144 --display 2          # macOS
  resctl toggle 1920x1080 2560x1440    # set list + switch immediately
  resctl toggle                        # cycle to next in saved list
  resctl get --json
  resctl list --json
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	cmd := strings.ToLower(os.Args[1])
	args := os.Args[2:]

	args, jsonOut, display, err := parseCommandFlags(args)
	if err != nil {
		fatalf("%v\n\n%s", err, usage)
	}

	switch cmd {
	case "list":
		err = cmdList(jsonOut, display)
	case "get":
		err = cmdGet(jsonOut, display)
	case "set":
		if len(args) == 0 {
			fatalf("set requires a resolution argument\n\n%s", usage)
		}
		err = cmdSet(args[0], display)
	case "toggle":
		err = cmdToggle(args, display)
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

func parseCommandFlags(args []string) (filtered []string, jsonOut bool, display string, err error) {
	filtered = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--display":
			i++
			if i >= len(args) {
				return nil, false, "", fmt.Errorf("--display requires a value")
			}
			display = args[i]
		case strings.HasPrefix(arg, "--display="):
			display = strings.TrimPrefix(arg, "--display=")
			if display == "" {
				return nil, false, "", fmt.Errorf("--display requires a value")
			}
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, jsonOut, display, nil
}

func cmdList(jsonOut bool, display string) error {
	modes, err := ListModes(display)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(modes)
	}

	cur, _ := GetCurrent(display)
	fmt.Println("Available resolutions:")
	for _, m := range modes {
		active := "  "
		if m.Width == cur.Width && m.Height == cur.Height && m.Freq == cur.Freq {
			active = "* "
		}
		fmt.Printf("  %s%dx%d @ %dHz\n", active, m.Width, m.Height, m.Freq)
	}
	return nil
}

func cmdGet(jsonOut bool, display string) error {
	cur, err := GetCurrent(display)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(cur)
	}
	fmt.Printf("Current: %dx%d @ %dHz\n", cur.Width, cur.Height, cur.Freq)
	return nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func cmdSet(arg, display string) error {
	width, height, freq, err := parseResolution(arg)
	if err != nil {
		return err
	}
	res, err := SetResolution(width, height, freq, display)
	if err != nil {
		return err
	}
	fmt.Printf("Set: %dx%d @ %dHz\n", res.Width, res.Height, res.Freq)
	return nil
}

func cmdToggle(args []string, display string) error {
	var state ToggleState

	if len(args) > 0 {
		state.Resolutions = args
		// Position the index at the current resolution so the first toggle
		// moves to the next one in the list.
		cur, _ := GetCurrent(display)
		curBase := fmt.Sprintf("%dx%d", cur.Width, cur.Height)
		for i, r := range args {
			base := strings.SplitN(strings.ToLower(strings.TrimSpace(r)), "@", 2)[0]
			if base == curBase {
				state.CurrentIndex = i
				break
			}
		}
	} else {
		var loadErr error
		state, loadErr = loadState(display)
		if loadErr != nil || len(state.Resolutions) == 0 {
			return fmt.Errorf("no toggle list configured — run: resctl toggle <res1> <res2>")
		}
	}

	state.CurrentIndex = (state.CurrentIndex + 1) % len(state.Resolutions)
	target := state.Resolutions[state.CurrentIndex]

	width, height, freq, err := parseResolution(target)
	if err != nil {
		return fmt.Errorf("invalid entry %q in toggle list: %w", target, err)
	}

	res, err := SetResolution(width, height, freq, display)
	if err != nil {
		return err
	}

	if saveErr := saveState(state, display); saveErr != nil {
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
