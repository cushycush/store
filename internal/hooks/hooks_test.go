package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/platform"
)

func TestRunEntryNilOrEmpty(t *testing.T) {
	t.Parallel()

	const (
		name   = "nvim"
		target = "~/.config/nvim"
		action = "link"
	)

	t.Run("nil hook entry", func(t *testing.T) {
		if err := RunEntry(t.TempDir(), name, target, action, "pre", nil); err != nil {
			t.Fatalf("RunEntry() error = %v", err)
		}
	})

	t.Run("empty hook command", func(t *testing.T) {
		h := &config.HookEntry{}
		if err := RunEntry(t.TempDir(), name, target, action, "pre", h); err != nil {
			t.Fatalf("RunEntry() error = %v", err)
		}
	})

	t.Run("unknown phase", func(t *testing.T) {
		h := &config.HookEntry{Pre: "printf ignored"}
		if err := RunEntry(t.TempDir(), name, target, action, "during", h); err == nil {
			t.Fatal("expected error for unknown phase, got nil")
		}
	})
}

func TestRunGlobalMissingHook(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := RunGlobal(root, "pre-store", "link"); err != nil {
		t.Fatalf("RunGlobal() error = %v", err)
	}
	assertNotExists(t, filepath.Join(root, "hook-env.txt"))
}

func writeScript(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod(%q) error = %v", path, err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	if !strings.Contains(string(data), want) {
		t.Fatalf("file %q does not contain %q\ncontents:\n%s", path, want, data)
	}
}

func assertTrimmedFileEquals(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	if got := strings.TrimSpace(string(data)); got != want {
		t.Fatalf("file %q = %q, want %q", path, got, want)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to not exist, got err = %v", path, err)
	}
}

func assertPlatformEnvVars(t *testing.T, path string) {
	t.Helper()

	for _, envVar := range platform.Detect().EnvVars() {
		assertFileContains(t, path, envVar)
	}
}
