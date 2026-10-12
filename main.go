package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// version is overridden at link time via -ldflags="-X main.version=vX.Y.Z".
var version = "dev"

// currentStateVersion is the schema version written to new/updated
// state.json files by saveState(). Bump this and add migration logic in
// loadState() whenever ToggleState's JSON shape changes in a
// backwards-incompatible way.
const currentStateVersion = 1

const usage = `resctl - display resolution manager

Usage:
  resctl list [--json] [--display <name>]           List available resolutions
  resctl get [--json] [--display <name>]            Show current resolution
  resctl set <WxH[@Hz]> [--display <name>] [--force] Set resolution
  resctl toggle [res1 res2 ...] [--display <name>]  Toggle between resolutions
  resctl toggle --list [--display <name>]           Show the saved toggle list
  resctl toggle --clear [--display <name>]          Delete the saved toggle list
  resctl displays [--json]       List connected displays and their current resolution
  resctl install                 Copy to ~/bin and add to PATH
  resctl uninstall [--force]     Remove from ~/bin (prompts for confirmation)
  resctl version                 Print version

Resolution format:  WxH  or  WxH@Hz  (e.g. 1920x1080  or  2560x1440@144)

Flags:
  --json              Output as JSON (supported by list and get)
  --display <name>    Target a specific display by name.
                      Linux: output name (e.g. DP-1, HDMI-1)
                      Windows: device name (e.g. \\.\DISPLAY1)
                      macOS: 1-based index (e.g. 1, 2)
  --force             Apply the change even if already at the target
                      resolution (set only). Skip the confirmation
                      prompt (uninstall only).
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
  resctl displays
  resctl displays --json
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
	case "list", "get", "set", "toggle":
		if flags.Display != "" {
			if verr := validateDisplay(flags.Display); verr != nil {
				fatalf("%v\n", verr)
			}
		}
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
	case "displays":
		err = cmdDisplays(flags.JSON)
	case "install":
		err = install()
	case "uninstall":
		err = uninstall(flags.Force)
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

// validateDisplay reports an error if display is non-empty and does not
// match any currently connected display, listing the available names to
// help the user correct a typo. If displays can't be enumerated for some
// reason, validation is skipped so the underlying command can surface its
// own (possibly more specific) error.
func validateDisplay(display string) error {
	displays, err := ListDisplays()
	if err != nil {
		return nil
	}
	return checkDisplayExists(display, displays)
}

// checkDisplayExists reports an error naming the available displays if
// display does not match any entry in displays. Split out from
// validateDisplay so the matching/formatting logic is testable without a
// real display backend.
func checkDisplayExists(display string, displays []DisplayInfo) error {
	for _, d := range displays {
		if d.Name == display {
			return nil
		}
	}
	names := make([]string, len(displays))
	for i, d := range displays {
		names[i] = d.Name
	}
	return fmt.Errorf("display %q not found; available displays: %s", display, strings.Join(names, ", "))
}

// cmdDisplays prints every connected display and its current resolution.
func cmdDisplays(jsonOut bool) error {
	displays, err := ListDisplays()
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSON(displays)
	}
	fmt.Println("Connected displays:")
	for _, d := range displays {
		fmt.Printf("  %s: %dx%d @ %dHz\n", d.Name, d.Current.Width, d.Current.Height, d.Current.Freq)
	}
	return nil
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
	for _, g := range groupModesByResolution(modes) {
		fmt.Printf("  %dx%d: ", g.Width, g.Height)
		for _, f := range g.Freqs {
			marker := ""
			if g.Width == cur.Width && g.Height == cur.Height && f == cur.Freq {
				marker = "*"
			}
			fmt.Printf("  %dHz%s", f, marker)
		}
		fmt.Println()
	}
	return nil
}

// modeGroup is a resolution together with the refresh rates available at it.
type modeGroup struct {
	Width  uint32
	Height uint32
	Freqs  []uint32
}

// groupModesByResolution groups a mode list by width x height, preserving
// the input order (ListModes returns modes sorted by width, then height,
// then freq, so freqs within each group come out sorted too).
func groupModesByResolution(modes []Resolution) []modeGroup {
	var groups []modeGroup
	for _, m := range modes {
		if n := len(groups); n > 0 && groups[n-1].Width == m.Width && groups[n-1].Height == m.Height {
			groups[n-1].Freqs = append(groups[n-1].Freqs, m.Freq)
			continue
		}
		groups = append(groups, modeGroup{Width: m.Width, Height: m.Height, Freqs: []uint32{m.Freq}})
	}
	return groups
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

// writeFileAtomic writes data to a temp file in the same directory as path,
// then renames it into place. This avoids leaving a truncated/corrupt file
// behind if the process is interrupted mid-write (os.Rename is atomic on
// the same filesystem on all platforms resctl supports).
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// confirm prompts the user with a y/N question on stdout and reads a
// single line of response from stdin. Only an explicit "y"/"yes" (any
// case) counts as confirmation; anything else, including EOF (e.g. stdin
// not a terminal), is treated as "no".
func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
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

// resolutionExistsInModes reports whether resStr (in the same format
// accepted by parseResolution, e.g. "1920x1080" or "1920x1080@144")
// matches any mode in modes. When resStr has no explicit refresh rate,
// any mode with a matching width/height counts as a match.
func resolutionExistsInModes(resStr string, modes []Resolution) bool {
	width, height, freq, err := parseResolution(resStr)
	if err != nil {
		return false
	}
	for _, m := range modes {
		if m.Width == width && m.Height == height && (freq == 0 || m.Freq == freq) {
			return true
		}
	}
	return false
}

// filterValidResolutions splits resolutions into the subset that matches at
// least one of modes (valid) and the rest (dropped), preserving order.
func filterValidResolutions(resolutions []string, modes []Resolution) (valid, dropped []string) {
	for _, r := range resolutions {
		if resolutionExistsInModes(r, modes) {
			valid = append(valid, r)
		} else {
			dropped = append(dropped, r)
		}
	}
	return valid, dropped
}

func cmdToggle(args []string, display string, list, clear bool) error {
	if (list || clear) && len(args) > 0 {
		return fmt.Errorf("--list/--clear cannot be combined with resolution arguments")
	}
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

		// The saved list may contain resolutions that no longer exist on
		// this display (e.g. after switching monitors or GPUs). Drop them
		// rather than failing deep inside SetResolution with a confusing
		// error. If ListModes itself fails, skip this check and let
		// SetResolution surface its own error, consistent with
		// validateDisplay's fail-open behavior elsewhere.
		if modes, modesErr := ListModes(display); modesErr == nil {
			valid, dropped := filterValidResolutions(state.Resolutions, modes)
			if len(valid) == 0 {
				return fmt.Errorf("no entries in the saved toggle list match this display's available resolutions — reconfigure with: resctl toggle <res1> <res2>")
			}
			if len(dropped) > 0 {
				fmt.Fprintf(os.Stderr, "warning: removing unavailable entries from toggle list: %s\n", strings.Join(dropped, ", "))
				if state.CurrentIndex >= len(valid) {
					state.CurrentIndex = 0
				}
				state.Resolutions = valid
			}
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
// current index. A missing state file means no list is configured yet; any
// other error (corrupt file, permission denied, etc.) is surfaced rather
// than silently treated the same way.
func printToggleList(display string) error {
	state, err := loadState(display)
	switch {
	case os.IsNotExist(err):
		fmt.Println("No toggle list configured — run: resctl toggle <res1> <res2>")
		return nil
	case err != nil:
		return fmt.Errorf("could not read toggle list: %w", err)
	}
	if len(state.Resolutions) == 0 {
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
