package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeExec drops an empty file at dir/name and marks it executable, so
// exec.LookPath finds it the way it'd find a real binary. Windows's LookPath
// has its own extension rules, so the tests that exercise real PATH lookup
// are skipped there.
func writeExec(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}

func TestResolveCompanion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec.LookPath extension handling differs on Windows; smoke-tested via CI on other OSes")
	}

	tmp := t.TempDir()
	// Strict git-convention binary.
	writeExec(t, tmp, "store-hello")
	// Bare binary name for a known companion.
	writeExec(t, tmp, "stock")
	// Bare binary name for something NOT whitelisted — must not resolve.
	writeExec(t, tmp, "random-tool")

	t.Setenv("PATH", tmp)

	tests := []struct {
		name   string
		sub    string
		wantOK bool
	}{
		{"flag is not a companion", "--help", false},
		{"empty string is not a companion", "", false},
		{"cobra-owned subcommand shadows companions", "apply", false},
		{"store-<sub> on PATH resolves", "hello", true},
		{"bare whitelisted companion resolves", "stock", true},
		{"bare non-whitelisted binary does not resolve", "random-tool", false},
		{"absent subcommand does not resolve", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := resolveCompanion(tt.sub)
			if ok != tt.wantOK {
				t.Fatalf("resolveCompanion(%q) ok = %v, want %v", tt.sub, ok, tt.wantOK)
			}
		})
	}
}

func TestResolveCompanionPrefersStrictPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("see TestResolveCompanion")
	}
	tmp := t.TempDir()
	// Both forms present. The strict prefix should win — that's the git
	// convention, and it means a user can override a bare companion by
	// dropping `store-stock` earlier on PATH.
	prefixed := writeExec(t, tmp, "store-stock")
	writeExec(t, tmp, "stock")

	t.Setenv("PATH", tmp)

	got, ok := resolveCompanion("stock")
	if !ok {
		t.Fatal("resolveCompanion(stock) returned !ok")
	}
	if got != prefixed {
		t.Fatalf("resolveCompanion(stock) = %q, want strict-prefix %q", got, prefixed)
	}
}
