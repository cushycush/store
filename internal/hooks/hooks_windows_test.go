//go:build windows

package hooks

import (
	"path/filepath"
	"testing"

	"github.com/cushycush/store/internal/config"
)

func TestRunGlobalWindows(t *testing.T) {
	t.Parallel()

	const (
		hookName = "pre-store.cmd"
		action   = "link"
	)

	tests := []struct {
		name    string
		setup   func(t *testing.T, root string)
		wantErr bool
		check   func(t *testing.T, root string)
	}{
		{
			name: "cmd script runs",
			setup: func(t *testing.T, root string) {
				writeScript(t,
					filepath.Join(root, ".store", "hooks", hookName),
					"@echo off\r\nset > \"%STORE_ROOT%\\hook-env.txt\"\r\ncd > \"%STORE_ROOT%\\hook-pwd.txt\"\r\n",
					0o644,
				)
			},
			check: func(t *testing.T, root string) {
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_ROOT="+root)
				assertFileContains(t, filepath.Join(root, "hook-env.txt"), "STORE_ACTION="+action)
				assertPlatformEnvVars(t, filepath.Join(root, "hook-env.txt"))
				assertTrimmedFileEquals(t, filepath.Join(root, "hook-pwd.txt"), root)
			},
		},
		{
			name: "cmd script fails",
			setup: func(t *testing.T, root string) {
				writeScript(t,
					filepath.Join(root, ".store", "hooks", hookName),
					"@echo off\r\nexit /b 1\r\n",
					0o644,
				)
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

func TestRunEntryWindows(t *testing.T) {
	t.Parallel()

	const (
		name   = "nvim"
		target = "~/.config/nvim"
		action = "link"
	)

	t.Run("pre hook runs via cmd.exe", func(t *testing.T) {
		root := t.TempDir()
		h := &config.HookEntry{Pre: `set > "%STORE_ROOT%\hook-env.txt"`}

		if err := RunEntry(root, name, target, action, "pre", h); err != nil {
			t.Fatalf("RunEntry() error = %v", err)
		}

		envFile := filepath.Join(root, "hook-env.txt")
		assertFileContains(t, envFile, "STORE_ROOT="+root)
		assertFileContains(t, envFile, "STORE_NAME="+name)
		assertFileContains(t, envFile, "STORE_TARGET="+target)
		assertFileContains(t, envFile, "STORE_ACTION="+action)
		assertPlatformEnvVars(t, envFile)
	})

	t.Run("post hook runs successfully", func(t *testing.T) {
		root := t.TempDir()
		h := &config.HookEntry{Post: `type nul > "%STORE_ROOT%\post-ran.txt"`}

		if err := RunEntry(root, name, target, action, "post", h); err != nil {
			t.Fatalf("RunEntry() error = %v", err)
		}
		assertExists(t, filepath.Join(root, "post-ran.txt"))
	})

	t.Run("failing command returns error", func(t *testing.T) {
		root := t.TempDir()
		h := &config.HookEntry{Pre: "exit /b 1"}

		if err := RunEntry(root, name, target, action, "pre", h); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
