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
  resctl set <WxH[@Hz]> [--display <name>] [--force] Set resolution
  resctl toggle [res1 res2 ...] [--display <name>]  Toggle between resolutions
  resctl toggle --list [--display <name>]           Show the saved toggle list
  resctl toggle --clear [--display <name>]          Delete the saved toggle list
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
  --force             Apply the change even if already at the target
                      resolution (set only).
  --list              Show the saved toggle list and current position
                      (toggle only).
  --clear             Delete the saved toggle list (toggle only).

Examples:
  resctl set 1920x1080
  resctl set 2560x1440@144
  resctl set 1920x1080 --force                  # reapply even if already active
  resctl set 2560x1440@144 --display DP-1       # Linux
  resctl set 2560x1440@144 --display \\.\DISPLAY2  # Windows
  resctl set 2560x1440@144 --display 2          # macOS
  resctl toggle 1920x1080 2560x1440    # set list + switch immediately
  resctl toggle                        # cycle to next in saved list
  resctl toggle --list                 # show the saved list + current index
  resctl toggle --clear                # delete the saved list
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

	args, flags, err := parseCommandFlags(args)
	if err != nil {
		fatalf("%v\n\n%s", err, usage)
	}

	switch cmd {
	case "list":
		err = cmdList(flags.JSON, flags.Display)
	case "get":
		err = cmdGet(flags.JSON, flags.Display)
	case "set":
		if len(args) == 0 {
			fatalf("set requires a resolution argument\n\n%s", usage)
		}
		err = cmdSet(args[0], flags.Display, flags.Force)
	case "toggle":
		err = cmdToggle(args, flags.Display, flags.List, flags.Clear)
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

// commandFlags holds the global flags shared across subcommands. Not every
// flag is meaningful for every command (e.g. --force only applies to set);
// unused flags are simply ignored by the commands that don't consume them.
type commandFlags struct {
	JSON    bool
	Display string
	Force   bool
	List    bool
	Clear   bool
}

func parseCommandFlags(args []string) (filtered []string, flags commandFlags, err error) {
	filtered = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			flags.JSON = true
		case arg == "--force":
			flags.Force = true
		case arg == "--list":
			flags.List = true
		case arg == "--clear":
			flags.Clear = true
		case arg == "--display":
			i++
			if i >= len(args) {
				return nil, commandFlags{}, fmt.Errorf("--display requires a value")
			}
			flags.Display = args[i]
		case strings.HasPrefix(arg, "--display="):
			flags.Display = strings.TrimPrefix(arg, "--display=")
			if flags.Display == "" {
				return nil, commandFlags{}, fmt.Errorf("--display requires a value")
			}
		default:
			filtered = append(filtered, arg)
		}
	}
	if flags.List && flags.Clear {
		return nil, commandFlags{}, fmt.Errorf("--list and --clear cannot be used together")
	}
	return filtered, flags, nil
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

func cmdSet(arg, display string, force bool) error {
	width, height, freq, err := parseResolution(arg)
	if err != nil {
		return err
	}

	if !force {
		if cur, curErr := GetCurrent(display); curErr == nil && isNoOp(cur, width, height, freq) {
			fmt.Printf("Already at %dx%d @ %dHz\n", cur.Width, cur.Height, cur.Freq)
			return nil
		}
	}

	res, err := SetResolution(width, height, freq, display)
	if err != nil {
		return err
	}
	fmt.Printf("Set: %dx%d @ %dHz\n", res.Width, res.Height, res.Freq)
	return nil
}

// isNoOp reports whether applying width/height/freq would have no visible
// effect given the display's current resolution cur. When freq is 0
// (unspecified by the user), only width and height are compared, since the
// current refresh rate will be preserved anyway.
func isNoOp(cur Resolution, width, height, freq uint32) bool {
	if cur.Width != width || cur.Height != height {
		return false
	}
	return freq == 0 || cur.Freq == freq
}

func cmdToggle(args []string, display string, list, clear bool) error {
	if clear {
		return clearToggleList(display)
	}
	if list {
		return printToggleList(display)
	}

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

// printToggleList prints the saved toggle list and marks the entry at the
// current index.
func printToggleList(display string) error {
	state, err := loadState(display)
	if err != nil || len(state.Resolutions) == 0 {
		fmt.Println("No toggle list configured — run: resctl toggle <res1> <res2>")
		return nil
	}

	fmt.Println("Toggle list:")
	for i, r := range state.Resolutions {
		marker := ""
		if i == state.CurrentIndex {
			marker = "  ← current"
		}
		fmt.Printf("  [%d] %s%s\n", i, r, marker)
	}
	return nil
}

// clearToggleList deletes the saved toggle list, if any.
func clearToggleList(display string) error {
	err := clearState(display)
	switch {
	case err == nil:
		fmt.Println("Toggle list cleared")
		return nil
	case os.IsNotExist(err):
		fmt.Println("No toggle list to clear")
		return nil
	default:
		return err
	}
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
