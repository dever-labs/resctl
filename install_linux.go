//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installDir() string {
	return filepath.Join(os.Getenv("HOME"), "bin")
}

// install copies the running binary to ~/bin/resctl and adds ~/bin to PATH
// in the user's shell rc file.
func install() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	dir := installDir()
	dest := filepath.Join(dir, "resctl")

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

	if err := addToShellPath(dir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: installed to %s but could not update PATH: %v\n", dest, err)
		fmt.Printf("Add %s to your PATH manually.\n", dir)
		return nil
	}

	fmt.Printf("Installed: %s\n", dest)
	fmt.Println("Restart your terminal for the PATH change to take effect.")
	return nil
}

// uninstall removes the binary from ~/bin.
func uninstall() error {
	dest := filepath.Join(installDir(), "resctl")
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

// addToShellPath appends ~/bin to PATH in the user's shell rc file.
func addToShellPath(dir string) error {
	rcFile := shellRCFile()
	if rcFile == "" {
		return fmt.Errorf("could not determine shell rc file")
	}

	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if strings.Contains(string(data), dir) {
		return nil // already present
	}

	if err := os.MkdirAll(filepath.Dir(rcFile), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(f, "\n# Added by resctl install\nexport PATH=\"$PATH:%s\"\n", dir)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

// shellRCFile returns the rc file for the current shell.
func shellRCFile() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	switch shell := os.Getenv("SHELL"); {
	case strings.HasSuffix(shell, "zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "bash"):
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".profile")
	}
}
