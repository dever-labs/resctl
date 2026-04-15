//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	hwndBroadcast   = uintptr(0xFFFF)
	wmSettingChange = 0x001A
	smtoAbortIfHung = 0x0002
)

var procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")

func installDir() string {
	return filepath.Join(os.Getenv("USERPROFILE"), "bin")
}

// install copies the running binary to ~/bin/resctl.exe and adds ~/bin to
// the user's PATH registry key.
func install() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	dir := installDir()
	dest := filepath.Join(dir, "resctl.exe")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}

	if !strings.EqualFold(exe, dest) {
		data, err := os.ReadFile(exe)
		if err != nil {
			return fmt.Errorf("could not read executable: %w", err)
		}
		if err := os.WriteFile(dest, data, 0o755); err != nil {
			return fmt.Errorf("could not write to %s: %w", dest, err)
		}
	}

	if err := addToUserPath(dir); err != nil {
		return fmt.Errorf("installed to %s but failed to update PATH: %w — add %s to your PATH manually", dest, err, dir)
	}

	fmt.Printf("Installed: %s\n", dest)
	fmt.Println("Restart your terminal for the PATH change to take effect.")
	return nil
}

// uninstall removes the binary from ~/bin.
func uninstall() error {
	dest := filepath.Join(installDir(), "resctl.exe")
	if err := os.Remove(dest); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("resctl is not installed.")
			return nil
		}
		return fmt.Errorf("could not remove %s: %w", dest, err)
	}
	fmt.Printf("Uninstalled: %s\n", dest)
	return nil
}

// addToUserPath appends dir to HKCU\Environment\PATH if not already present,
// then broadcasts WM_SETTINGCHANGE so open terminals pick up the new value.
func addToUserPath(dir string) error {
	current := getUserPath()

	dirLower := strings.ToLower(dir)
	for _, p := range strings.Split(current, ";") {
		if strings.ToLower(strings.TrimSpace(p)) == dirLower {
			return nil // already in PATH
		}
	}

	var newPath string
	if current == "" {
		newPath = dir
	} else {
		newPath = current + ";" + dir
	}

	_, err := exec.Command(
		"reg", "add", `HKCU\Environment`,
		"/v", "PATH", "/t", "REG_EXPAND_SZ", "/d", newPath, "/f",
	).Output()
	if err != nil {
		return err
	}

	// Notify the shell so new terminals inherit the updated PATH.
	env, _ := syscall.UTF16PtrFromString("Environment")
	_, _, _ = procSendMessageTimeoutW.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(env)),
		smtoAbortIfHung,
		5000,
		0,
	)

	fmt.Printf("Added %s to user PATH\n", dir)
	return nil
}

// getUserPath reads the current PATH value from HKCU\Environment via reg.exe.
func getUserPath() string {
	out, err := exec.Command("reg", "query", `HKCU\Environment`, "/v", "PATH").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "PATH") {
			// line looks like: "PATH    REG_EXPAND_SZ    C:\foo;C:\bar"
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return strings.Join(parts[2:], " ")
			}
		}
	}
	return ""
}
