//go:build darwin

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

// addToShellPath appends dir to PATH in the user's shell rc file.
// Fish shell uses a different config location and PATH syntax.
func addToShellPath(dir string) error {
	rcFile := shellRCFile()
	if rcFile == "" {
		return fmt.Errorf("could not determine shell rc file")
	}

	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	isFish := strings.HasSuffix(os.Getenv("SHELL"), "fish")

	content := string(data)
	if isFish {
		if strings.Contains(content, "fish_add_path") && strings.Contains(content, dir) {
			return nil
		}
	} else {
		if strings.Contains(content, "PATH:"+dir+"\"") ||
			strings.Contains(content, "PATH:"+dir+":") ||
			strings.Contains(content, "PATH:"+dir+"\n") {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(rcFile), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	var writeErr error
	if isFish {
		_, writeErr = fmt.Fprintf(f, "\n# Added by resctl install\nfish_add_path %s\n", dir)
	} else {
		_, writeErr = fmt.Fprintf(f, "\n# Added by resctl install\nexport PATH=\"$PATH:%s\"\n", dir)
	}
	if closeErr := f.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	return writeErr
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
		// macOS terminals open login shells by default, which source
		// ~/.bash_profile, not ~/.bashrc.
		return filepath.Join(home, ".bash_profile")
	case strings.HasSuffix(shell, "fish"):
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return filepath.Join(home, ".profile")
	}
}
