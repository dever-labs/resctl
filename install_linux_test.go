//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellRCFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		shell string
		want  string
	}{
		{"/usr/bin/fish", filepath.Join(home, ".config", "fish", "config.fish")},
		{"/usr/local/bin/fish", filepath.Join(home, ".config", "fish", "config.fish")},
		{"/bin/zsh", filepath.Join(home, ".zshrc")},
		{"/usr/bin/zsh", filepath.Join(home, ".zshrc")},
		{"/bin/bash", filepath.Join(home, ".bashrc")},
		{"/usr/bin/bash", filepath.Join(home, ".bashrc")},
		{"/bin/sh", filepath.Join(home, ".profile")},
		{"", filepath.Join(home, ".profile")},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			t.Setenv("SHELL", tt.shell)
			if got := shellRCFile(); got != tt.want {
				t.Errorf("shellRCFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellRCFile_NoHome(t *testing.T) {
	t.Setenv("HOME", "")
	if got := shellRCFile(); got != "" {
		t.Errorf("shellRCFile() with no HOME = %q, want empty", got)
	}
}

func TestAddToShellPath_Fish(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/bin/fish")
	binDir := filepath.Join(home, "bin")

	if err := addToShellPath(binDir); err != nil {
		t.Fatalf("addToShellPath: %v", err)
	}

	rcFile := filepath.Join(home, ".config", "fish", "config.fish")
	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("read config.fish: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "fish_add_path") {
		t.Errorf("config.fish missing fish_add_path:\n%s", content)
	}
	if strings.Contains(content, "export PATH") {
		t.Errorf("config.fish must not contain export PATH (invalid fish syntax):\n%s", content)
	}
	if !strings.Contains(content, binDir) {
		t.Errorf("config.fish missing %q:\n%s", binDir, content)
	}
}

func TestAddToShellPath_Bash(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	binDir := filepath.Join(home, "bin")

	if err := addToShellPath(binDir); err != nil {
		t.Fatalf("addToShellPath: %v", err)
	}

	rcFile := filepath.Join(home, ".bashrc")
	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("read .bashrc: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "export PATH") {
		t.Errorf(".bashrc missing export PATH:\n%s", content)
	}
	if !strings.Contains(content, binDir) {
		t.Errorf(".bashrc missing %q:\n%s", binDir, content)
	}
}

func TestAddToShellPath_Idempotent(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/usr/bin/fish"} {
		t.Run(shell, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("SHELL", shell)
			binDir := filepath.Join(home, "bin")

			for i := 0; i < 3; i++ {
				if err := addToShellPath(binDir); err != nil {
					t.Fatalf("call %d: %v", i+1, err)
				}
			}

			rcFile := shellRCFile()
			data, err := os.ReadFile(rcFile)
			if err != nil {
				t.Fatalf("read rc file: %v", err)
			}
			if count := strings.Count(string(data), binDir); count != 1 {
				t.Errorf("%q appears %d times in rc file, want 1:\n%s", binDir, count, data)
			}
		})
	}
}
