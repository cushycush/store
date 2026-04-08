package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/platform"
)

func TestRunGlobal(t *testing.T) {
	t.Parallel()

	const (
		hookName = "pre-store"
		action   = "link"
	)

	tests := []struct {
		name    string
		setup   func(t *testing.T, root string)
		wantErr bool
		check   func(t *testing.T, root string)
	}{
		{
			name: "hook exists and is executable",
			setup: func(t *testing.T, root string) {
				writeScript(t, filepath.Join(root, ".store", "hooks", hookName), "#!/bin/sh\nenv > \"$STORE_ROOT/hook-env.txt\"\npwd > \"$STORE_ROOT/hook-pwd.txt\"\n", 0o755)
			},
			check: func(t *testing.T, root string) {
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_ROOT="+root)
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_ACTION="+action)
				assertPlatformEnvVars(t, filepath.Join(root, "hook-env.txt"))
				assertTrimmedFileEquals(t, filepath.Join(root, "hook-pwd.txt"), root)
			},
		},
		{
			name: "hook does not exist",
			check: func(t *testing.T, root string) {
				assertNotExists(t, filepath.Join(root, "hook-env.txt"))
			},
		},
		{
			name: "hook exists but is not executable",
			setup: func(t *testing.T, root string) {
				writeScript(t, filepath.Join(root, ".store", "hooks", hookName), "#!/bin/sh\nenv > \"$STORE_ROOT/hook-env.txt\"\n", 0o644)
			},
			check: func(t *testing.T, root string) {
				assertNotExists(t, filepath.Join(root, "hook-env.txt"))
			},
		},
		{
			name: "hook fails",
			setup: func(t *testing.T, root string) {
				writeScript(t, filepath.Join(root, ".store", "hooks", hookName), "#!/bin/sh\nexit 1\n", 0o755)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			if tt.setup != nil {
				tt.setup(t, root)
			}

			err := RunGlobal(root, hookName, action)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("RunGlobal() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root)
			}
		})
	}
}

func TestRunEntry(t *testing.T) {
	t.Parallel()

	const (
		name   = "nvim"
		target = "~/.config/nvim"
		action = "link"
	)

	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) *config.HookEntry
		invoke  func(t *testing.T, root string, h *config.HookEntry) error
		wantErr bool
		check   func(t *testing.T, root string)
	}{
		{
			name: "pre hook runs successfully",
			setup: func(t *testing.T, root string) *config.HookEntry {
				path := filepath.Join(root, "pre-hook.sh")
				writeScript(t, path, "#!/bin/sh\nenv > \"$STORE_ROOT/hook-env.txt\"\n", 0o755)
				return &config.HookEntry{Pre: path}
			},
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				return RunEntry(root, name, target, action, "pre", h)
			},
			check: func(t *testing.T, root string) {
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_ROOT="+root)
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_NAME="+name)
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_TARGET="+target)
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_ACTION="+action)
				assertPlatformEnvVars(t, filepath.Join(root, "hook-env.txt"))
			},
		},
		{
			name: "post hook runs successfully",
			setup: func(t *testing.T, root string) *config.HookEntry {
				path := filepath.Join(root, "post-hook.sh")
				writeScript(t, path, "#!/bin/sh\ntouch \"$STORE_ROOT/post-ran.txt\"\n", 0o755)
				return &config.HookEntry{Post: path}
			},
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				return RunEntry(root, name, target, action, "post", h)
			},
			check: func(t *testing.T, root string) {
				assertExists(t, filepath.Join(root, "post-ran.txt"))
			},
		},
		{
			name: "nil hook entry",
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				return RunEntry(root, name, target, action, "pre", h)
			},
		},
		{
			name: "empty hook command",
			setup: func(t *testing.T, root string) *config.HookEntry {
				return &config.HookEntry{}
			},
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				return RunEntry(root, name, target, action, "pre", h)
			},
		},
		{
			name: "hook fails",
			setup: func(t *testing.T, root string) *config.HookEntry {
				path := filepath.Join(root, "fail-hook.sh")
				writeScript(t, path, "#!/bin/sh\nexit 1\n", 0o755)
				return &config.HookEntry{Pre: path}
			},
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				return RunEntry(root, name, target, action, "pre", h)
			},
			wantErr: true,
		},
		{
			name: "unknown phase",
			setup: func(t *testing.T, root string) *config.HookEntry {
				return &config.HookEntry{Pre: "printf ignored"}
			},
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				return RunEntry(root, name, target, action, "during", h)
			},
			wantErr: true,
		},
		{
			name: "pre empty but post set",
			setup: func(t *testing.T, root string) *config.HookEntry {
				path := filepath.Join(root, "only-post-hook.sh")
				writeScript(t, path, "#!/bin/sh\ntouch \"$STORE_ROOT/post-only-ran.txt\"\n", 0o755)
				return &config.HookEntry{Post: path}
			},
			invoke: func(t *testing.T, root string, h *config.HookEntry) error {
				if err := RunEntry(root, name, target, action, "pre", h); err != nil {
					return err
				}
				return RunEntry(root, name, target, action, "post", h)
			},
			check: func(t *testing.T, root string) {
				assertExists(t, filepath.Join(root, "post-only-ran.txt"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			var hooks *config.HookEntry
			if tt.setup != nil {
				hooks = tt.setup(t, root)
			}

			invoke := tt.invoke
			if invoke == nil {
				invoke = func(t *testing.T, root string, h *config.HookEntry) error {
					return RunEntry(root, name, target, action, "pre", h)
				}
			}

			err := invoke(t, root, hooks)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("RunEntry() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root)
			}
		})
	}
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
