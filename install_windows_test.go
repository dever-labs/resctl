//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDir(t *testing.T) {
	want := filepath.Join(os.Getenv("USERPROFILE"), "bin")
	if got := installDir(); got != want {
		t.Errorf("installDir() = %q, want %q", got, want)
	}
}

// TestAddToUserPath_Idempotent verifies that calling addToUserPath with a
// directory that is already in HKCU\Environment\PATH is a no-op and does not
// duplicate the entry.
func TestAddToUserPath_Idempotent(t *testing.T) {
	current := getUserPath()
	if current == "" {
		t.Skip("HKCU\\Environment PATH is empty; skipping idempotency check")
	}

	// Pick the first non-empty existing entry — addToUserPath must not add it again.
	var existing string
	for _, p := range strings.Split(current, ";") {
		if p = strings.TrimSpace(p); p != "" {
			existing = p
			break
		}
	}
	if existing == "" {
		t.Skip("no non-empty PATH entries found; skipping")
	}

	for i := 0; i < 3; i++ {
		if err := addToUserPath(existing); err != nil {
			t.Fatalf("call %d addToUserPath(%q): %v", i+1, existing, err)
		}
	}

	after := getUserPath()
	count := 0
	for _, e := range strings.Split(after, ";") {
		if strings.EqualFold(strings.TrimSpace(e), existing) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%q appears %d times in PATH after 3 idempotent calls, want 1:\n%s", existing, count, after)
	}
}

// TestAddToUserPath_NewEntry verifies that a new directory is appended to
// HKCU\Environment\PATH exactly once, and that re-running is idempotent.
func TestAddToUserPath_NewEntry(t *testing.T) {
	testDir := filepath.Join(os.Getenv("USERPROFILE"), "bin", "resctl-ci-test")

	// Clean up the test entry from the registry regardless of test outcome.
	t.Cleanup(func() { removeFromUserPath(t, testDir) })

	for i := 0; i < 2; i++ {
		if err := addToUserPath(testDir); err != nil {
			t.Fatalf("call %d addToUserPath(%q): %v", i+1, testDir, err)
		}
	}

	after := getUserPath()
	count := 0
	for _, e := range strings.Split(after, ";") {
		if strings.EqualFold(strings.TrimSpace(e), testDir) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%q appears %d times in PATH after 2 calls, want 1:\n%s", testDir, count, after)
	}
}

// removeFromUserPath removes dir from HKCU\Environment\PATH. Used only in
// test cleanup — best-effort, failures are logged but do not fail the test.
func removeFromUserPath(t *testing.T, dir string) {
	t.Helper()
	current := getUserPath()
	if current == "" {
		return
	}
	var kept []string
	for _, p := range strings.Split(current, ";") {
		if !strings.EqualFold(strings.TrimSpace(p), dir) {
			kept = append(kept, p)
		}
	}
	newPath := strings.Join(kept, ";")
	if err := exec.Command(
		"reg", "add", `HKCU\Environment`,
		"/v", "PATH", "/t", "REG_EXPAND_SZ", "/d", newPath, "/f",
	).Run(); err != nil {
		t.Logf("cleanup: failed to remove %q from PATH: %v", dir, err)
	}
}
