//go:build !windows

package hooks

import (
	"path/filepath"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
)

func TestRunGlobalUnix(t *testing.T) {
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
				// macOS `pwd` prints the canonicalized path (/private/var/...)
				// while t.TempDir() returns the symlinked /var/... form.
				canonical, err := filepath.EvalSymlinks(root)
				if err != nil {
					t.Fatalf("EvalSymlinks(%q) error = %v", root, err)
				}
				assertTrimmedFileEquals(t, filepath.Join(root, "hook-pwd.txt"), canonical)
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

func TestRunEntryUnix(t *testing.T) {
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

			err := tt.invoke(t, root, hooks)
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
