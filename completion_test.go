package main

import (
	"strings"
	"testing"
)

func TestCmdCompletion_Bash(t *testing.T) {
	script, err := cmdCompletion("bash")
	if err != nil {
		t.Fatalf("cmdCompletion(bash): %v", err)
	}
	if !strings.Contains(script, "_resctl()") || !strings.Contains(script, "complete -F _resctl resctl") {
		t.Errorf("bash script missing expected markers:\n%s", script)
	}
	for _, cmd := range completionCommands {
		if !strings.Contains(script, cmd) {
			t.Errorf("bash script missing command %q", cmd)
		}
	}
}

func TestCmdCompletion_Zsh(t *testing.T) {
	script, err := cmdCompletion("zsh")
	if err != nil {
		t.Fatalf("cmdCompletion(zsh): %v", err)
	}
	if !strings.HasPrefix(script, "#compdef resctl") {
		t.Errorf("zsh script should start with #compdef resctl directive:\n%s", script)
	}
	for _, flag := range completionFlags {
		if !strings.Contains(script, flag) {
			t.Errorf("zsh script missing flag %q", flag)
		}
	}
}

func TestCmdCompletion_Fish(t *testing.T) {
	script, err := cmdCompletion("fish")
	if err != nil {
		t.Fatalf("cmdCompletion(fish): %v", err)
	}
	for _, cmd := range completionCommands {
		if !strings.Contains(script, "-a "+cmd) {
			t.Errorf("fish script missing completion for command %q", cmd)
		}
	}
	for _, flag := range completionFlags {
		if !strings.Contains(script, "-l "+trimDashes(flag)) {
			t.Errorf("fish script missing completion for flag %q", flag)
		}
	}
}

func TestCmdCompletion_UnsupportedShell(t *testing.T) {
	_, err := cmdCompletion("powershell")
	if err == nil {
		t.Fatal("cmdCompletion(powershell): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "powershell") {
		t.Errorf("error = %q, want it to mention the unsupported shell name", err.Error())
	}
}

func TestTrimDashes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"--display", "display"},
		{"-f", "f"},
		{"json", "json"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := trimDashes(tt.in); got != tt.want {
			t.Errorf("trimDashes(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
