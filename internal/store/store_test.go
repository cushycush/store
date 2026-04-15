package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/linker"
)

func createStore(t *testing.T, root, name string, files map[string]string) string {
	t.Helper()

	storeDir := filepath.Join(root, name)
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", storeDir, err)
	}

	for rel, content := range files {
		writeTestFile(t, filepath.Join(storeDir, rel), content)
	}

	return storeDir
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func mustSymlink(t *testing.T, target, linkPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(linkPath), err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink(%q, %q): %v", target, linkPath, err)
	}
}

func assertSymlinkPointsTo(t *testing.T, linkPath, wantTarget string) {
	t.Helper()

	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", linkPath, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%q is not a symlink", linkPath)
	}

	gotTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink(%q): %v", linkPath, err)
	}

	gotAbs, err := filepath.Abs(gotTarget)
	if err != nil {
		t.Fatalf("Abs(%q): %v", gotTarget, err)
	}
	wantAbs, err := filepath.Abs(wantTarget)
	if err != nil {
		t.Fatalf("Abs(%q): %v", wantTarget, err)
	}

	if gotAbs != wantAbs {
		t.Fatalf("symlink %q points to %q, want %q", linkPath, gotAbs, wantAbs)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be missing, got err=%v", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("content of %q = %q, want %q", path, string(data), want)
	}
}

func statusKey(info StatusInfo) string {
	return fmt.Sprintf("%s|%s|%s", info.Name, info.File, info.Target)
}

func statusMap(results []StatusInfo) map[string]StatusInfo {
	m := make(map[string]StatusInfo, len(results))
	for _, info := range results {
		m[statusKey(info)] = info
	}
	return m
}

func TestStoreTarget(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) config.TargetEntry
		wantErr string
		check   func(t *testing.T, root string, te config.TargetEntry)
	}{
		{
			name: "whole directory mode creates symlink",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				return config.TargetEntry{Target: filepath.Join(root, "targets", "app")}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, te.Target, filepath.Join(root, "app"))
			},
		},
		{
			name: "file mode creates per file symlinks",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"a.txt":           "a",
					"nested/b.txt":    "b",
					"nested/c.conf":   "c",
					"other/skip.json": "skip",
				})
				return config.TargetEntry{
					Target: filepath.Join(root, "targets", "files"),
					Files:  []string{"a.txt", "nested/b.txt"},
				}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "a.txt"), filepath.Join(root, "app", "a.txt"))
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "nested", "b.txt"), filepath.Join(root, "app", "nested", "b.txt"))
				if fi, err := os.Lstat(te.Target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("file-mode target root %q should not be a symlink", te.Target)
				}
			},
		},
		{
			name: "file mode with patterns links matched files only",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"top.conf":      "top",
					"nested/a.conf": "nested",
					"nested/b.txt":  "skip",
				})
				return config.TargetEntry{
					Target:   filepath.Join(root, "targets", "patterns"),
					Patterns: []string{"*.conf", "**/*.conf"},
				}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "top.conf"), filepath.Join(root, "app", "top.conf"))
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "nested", "a.conf"), filepath.Join(root, "app", "nested", "a.conf"))
				assertMissing(t, filepath.Join(te.Target, "nested", "b.txt"))
			},
		},
		{
			name: "whole directory mode auto promotes when global ignores are present",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"config.txt":         "data",
					"nested/keep.txt":    "keep",
					".store/config.yaml": "ignored",
				})
				return config.TargetEntry{Target: filepath.Join(root, "targets", "auto-promote")}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "config.txt"), filepath.Join(root, "app", "config.txt"))
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "nested", "keep.txt"), filepath.Join(root, "app", "nested", "keep.txt"))
				assertMissing(t, filepath.Join(te.Target, ".store"))
				if fi, err := os.Lstat(te.Target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("auto-promoted target root %q should not be a symlink", te.Target)
				}
			},
		},
		{
			name: "explicit ignore auto promotes and excludes matching files",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"keep.txt":  "keep",
					"notes.bak": "bak",
				})
				return config.TargetEntry{
					Target: filepath.Join(root, "targets", "ignore"),
					Ignore: []string{"*.bak"},
				}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "keep.txt"), filepath.Join(root, "app", "keep.txt"))
				assertMissing(t, filepath.Join(te.Target, "notes.bak"))
				if fi, err := os.Lstat(te.Target); err == nil && fi.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("ignored target root %q should not be a symlink", te.Target)
				}
			},
		},
		{
			name: "file mode and ignore both apply",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"keep.txt":         "keep",
					"skip.bak":         "skip",
					"nested/app.conf":  "conf",
					"ignore/skip.conf": "ignored",
				})
				return config.TargetEntry{
					Target:   filepath.Join(root, "targets", "file-ignore"),
					Files:    []string{"keep.txt", "skip.bak"},
					Patterns: []string{"**/*.conf"},
					Ignore:   []string{"*.bak", "ignore/"},
				}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "keep.txt"), filepath.Join(root, "app", "keep.txt"))
				assertSymlinkPointsTo(t, filepath.Join(te.Target, "nested", "app.conf"), filepath.Join(root, "app", "nested", "app.conf"))
				assertMissing(t, filepath.Join(te.Target, "skip.bak"))
				assertMissing(t, filepath.Join(te.Target, "ignore"))
			},
		},
		{
			name: "returns error when target expansion fails",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				t.Setenv("HOME", "")
				if _, err := os.UserHomeDir(); err == nil {
					t.Skip("cannot force os.UserHomeDir failure on this platform")
				}
				return config.TargetEntry{Target: "~"}
			},
			wantErr: "store \"app\" target \"~\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			te := tt.setup(t, root)

			err := StoreTarget(root, "app", te)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("StoreTarget() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("StoreTarget() error = %v", err)
			}
			if tt.check != nil {
				tt.check(t, root, te)
			}
		})
	}
}

func TestStore(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) config.StoreEntry
		wantErr string
		check   func(t *testing.T, root string, entry config.StoreEntry)
	}{
		{
			name: "single target entry links store",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				return config.StoreEntry{Target: filepath.Join(root, "targets", "app")}
			},
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, entry.Target, filepath.Join(root, "app"))
			},
		},
		{
			name: "multi target entry links all targets",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"config.txt":    "config",
					"nested/a.txt":  "a",
					"nested/b.conf": "b",
				})
				return config.StoreEntry{Targets: []config.TargetEntry{
					{Target: filepath.Join(root, "targets", "whole")},
					{Target: filepath.Join(root, "targets", "files"), Files: []string{"nested/a.txt"}},
				}}
			},
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, entry.Targets[0].Target, filepath.Join(root, "app"))
				assertSymlinkPointsTo(t, filepath.Join(entry.Targets[1].Target, "nested", "a.txt"), filepath.Join(root, "app", "nested", "a.txt"))
			},
		},
		{
			name: "entry with no targets is a no op",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				return config.StoreEntry{}
			},
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				if _, err := os.ReadDir(filepath.Join(root, "targets")); !os.IsNotExist(err) {
					t.Fatalf("expected no target output, got err=%v", err)
				}
			},
		},
		{
			name: "pre hook failure aborts store",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				// `exit 1` is valid in both sh -c and cmd.exe /C, so this
				// exercises the abort path on every platform without needing
				// a shell-script file with a platform-specific shebang.
				return config.StoreEntry{
					Target: filepath.Join(root, "targets", "app"),
					Hooks:  &config.HookEntry{Pre: "exit 1"},
				}
			},
			wantErr: "hook pre (app) failed",
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertMissing(t, entry.Target)
			},
		},
		{
			name: "post hook failure warns but succeeds",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				return config.StoreEntry{
					Target: filepath.Join(root, "targets", "app"),
					Hooks:  &config.HookEntry{Post: "exit 1"},
				}
			},
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, entry.Target, filepath.Join(root, "app"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			entry := tt.setup(t, root)

			err := Store(root, "app", entry)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Store() error = %v, want substring %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Store() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root, entry)
			}
		})
	}
}

func TestStoreAll(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) *config.Config
		wantErr string
		check   func(t *testing.T, root string, cfg *config.Config)
	}{
		{
			name: "multiple stores all succeed",
			setup: func(t *testing.T, root string) *config.Config {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				createStore(t, root, "shells", map[string]string{".zshrc": "zsh"})
				return &config.Config{Stores: map[string]config.StoreEntry{
					"app":    {Target: filepath.Join(root, "targets", "app")},
					"shells": {Target: filepath.Join(root, "targets", "home"), Files: []string{".zshrc"}},
				}}
			},
			check: func(t *testing.T, root string, cfg *config.Config) {
				t.Helper()
				assertSymlinkPointsTo(t, cfg.Stores["app"].Target, filepath.Join(root, "app"))
				assertSymlinkPointsTo(t, filepath.Join(cfg.Stores["shells"].Target, ".zshrc"), filepath.Join(root, "shells", ".zshrc"))
			},
		},
		{
			name: "partial failure still stores successful entries",
			setup: func(t *testing.T, root string) *config.Config {
				t.Helper()
				createStore(t, root, "good", map[string]string{"config.txt": "good"})
				createStore(t, root, "bad", map[string]string{"config.txt": "bad"})
				writeTestFile(t, filepath.Join(root, "targets", "bad"), "conflict")
				return &config.Config{Stores: map[string]config.StoreEntry{
					"good": {Target: filepath.Join(root, "targets", "good")},
					"bad":  {Target: filepath.Join(root, "targets", "bad")},
				}}
			},
			wantErr: "1 store(s) failed",
			check: func(t *testing.T, root string, cfg *config.Config) {
				t.Helper()
				assertSymlinkPointsTo(t, cfg.Stores["good"].Target, filepath.Join(root, "good"))
				assertFileContent(t, cfg.Stores["bad"].Target, "conflict")
			},
		},
		{
			name: "empty config returns error",
			setup: func(t *testing.T, root string) *config.Config {
				t.Helper()
				return &config.Config{}
			},
			wantErr: "no stores defined in config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := tt.setup(t, root)

			err := StoreAll(root, cfg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("StoreAll() error = %v, want substring %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("StoreAll() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root, cfg)
			}
		})
	}
}

func TestStoreRemoveTarget(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) config.TargetEntry
		wantErr string
		check   func(t *testing.T, root string, te config.TargetEntry)
	}{
		{
			name: "removes whole directory symlink",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				te := config.TargetEntry{Target: filepath.Join(root, "targets", "app")}
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				if err := StoreTarget(root, "app", te); err != nil {
					t.Fatalf("StoreTarget() setup error: %v", err)
				}
				return te
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertMissing(t, te.Target)
			},
		},
		{
			name: "removes file mode symlinks and cleans empty directories",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				te := config.TargetEntry{
					Target: filepath.Join(root, "targets", "files"),
					Files:  []string{"deep/a.txt", "deep/nested/b.txt"},
				}
				createStore(t, root, "app", map[string]string{
					"deep/a.txt":        "a",
					"deep/nested/b.txt": "b",
				})
				if err := StoreTarget(root, "app", te); err != nil {
					t.Fatalf("StoreTarget() setup error: %v", err)
				}
				return te
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertMissing(t, filepath.Join(te.Target, "deep", "a.txt"))
				assertMissing(t, filepath.Join(te.Target, "deep", "nested", "b.txt"))
				assertMissing(t, filepath.Join(te.Target, "deep", "nested"))
				assertMissing(t, filepath.Join(te.Target, "deep"))
				assertMissing(t, te.Target)
			},
		},
		{
			name: "missing targets are ignored",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"a.txt": "a"})
				return config.TargetEntry{Target: filepath.Join(root, "targets", "missing"), Files: []string{"a.txt"}}
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertMissing(t, te.Target)
			},
		},
		{
			name: "conflicting file mode targets are skipped",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				te := config.TargetEntry{Target: filepath.Join(root, "targets", "files"), Files: []string{"conflict.txt"}}
				createStore(t, root, "app", map[string]string{"conflict.txt": "source"})
				writeTestFile(t, filepath.Join(te.Target, "conflict.txt"), "target")
				return te
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertFileContent(t, filepath.Join(te.Target, "conflict.txt"), "target")
			},
		},
		{
			name: "auto promoted targets remove linked files",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				te := config.TargetEntry{Target: filepath.Join(root, "targets", "auto-remove")}
				createStore(t, root, "app", map[string]string{
					"keep.txt":           "keep",
					".store/config.yaml": "ignored",
				})
				if err := StoreTarget(root, "app", te); err != nil {
					t.Fatalf("StoreTarget() setup error: %v", err)
				}
				return te
			},
			check: func(t *testing.T, root string, te config.TargetEntry) {
				t.Helper()
				assertMissing(t, filepath.Join(te.Target, "keep.txt"))
				assertMissing(t, te.Target)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			te := tt.setup(t, root)

			err := StoreRemoveTarget(root, "app", te)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("StoreRemoveTarget() error = %v, want substring %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("StoreRemoveTarget() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root, te)
			}
		})
	}
}

func TestStoreRemove(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) config.StoreEntry
		wantErr string
		check   func(t *testing.T, root string, entry config.StoreEntry)
	}{
		{
			name: "removes all targets for a store",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				entry := config.StoreEntry{Targets: []config.TargetEntry{
					{Target: filepath.Join(root, "targets", "whole")},
					{Target: filepath.Join(root, "targets", "files"), Files: []string{"nested/a.txt"}},
				}}
				createStore(t, root, "app", map[string]string{
					"config.txt":   "data",
					"nested/a.txt": "a",
				})
				if err := Store(root, "app", entry); err != nil {
					t.Fatalf("Store() setup error: %v", err)
				}
				return entry
			},
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertMissing(t, entry.Targets[0].Target)
				assertMissing(t, entry.Targets[1].Target)
			},
		},
		{
			name: "pre hook failure aborts removal",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				base := config.StoreEntry{Target: filepath.Join(root, "targets", "app")}
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				if err := Store(root, "app", base); err != nil {
					t.Fatalf("Store() setup error: %v", err)
				}
				base.Hooks = &config.HookEntry{Pre: "exit 1"}
				return base
			},
			wantErr: "hook pre (app) failed",
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertSymlinkPointsTo(t, entry.Target, filepath.Join(root, "app"))
			},
		},
		{
			name: "post hook failure warns after removal",
			setup: func(t *testing.T, root string) config.StoreEntry {
				t.Helper()
				base := config.StoreEntry{Target: filepath.Join(root, "targets", "app")}
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				if err := Store(root, "app", base); err != nil {
					t.Fatalf("Store() setup error: %v", err)
				}
				base.Hooks = &config.HookEntry{Post: "exit 1"}
				return base
			},
			check: func(t *testing.T, root string, entry config.StoreEntry) {
				t.Helper()
				assertMissing(t, entry.Target)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			entry := tt.setup(t, root)

			err := StoreRemove(root, "app", entry)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("StoreRemove() error = %v, want substring %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("StoreRemove() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root, entry)
			}
		})
	}
}

func TestStoreRemoveAll(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) *config.Config
		wantErr string
		check   func(t *testing.T, root string, cfg *config.Config)
	}{
		{
			name: "removes all stores",
			setup: func(t *testing.T, root string) *config.Config {
				t.Helper()
				cfg := &config.Config{Stores: map[string]config.StoreEntry{
					"app":    {Target: filepath.Join(root, "targets", "app")},
					"shells": {Target: filepath.Join(root, "targets", "home"), Files: []string{".zshrc"}},
				}}
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				createStore(t, root, "shells", map[string]string{".zshrc": "zsh"})
				if err := StoreAll(root, cfg); err != nil {
					t.Fatalf("StoreAll() setup error: %v", err)
				}
				return cfg
			},
			check: func(t *testing.T, root string, cfg *config.Config) {
				t.Helper()
				assertMissing(t, cfg.Stores["app"].Target)
				assertMissing(t, cfg.Stores["shells"].Target)
			},
		},
		{
			name: "empty config returns error",
			setup: func(t *testing.T, root string) *config.Config {
				t.Helper()
				return &config.Config{}
			},
			wantErr: "no stores defined in config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := tt.setup(t, root)

			err := StoreRemoveAll(root, cfg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("StoreRemoveAll() error = %v, want substring %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("StoreRemoveAll() error = %v", err)
			}

			if tt.check != nil {
				tt.check(t, root, cfg)
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	t.Run("whole directory statuses", func(t *testing.T) {
		tests := []struct {
			name       string
			setup      func(t *testing.T, root, source, target string)
			wantStatus linker.Status
		}{
			{
				name: "linked",
				setup: func(t *testing.T, root, source, target string) {
					t.Helper()
					if err := StoreTarget(root, "app", config.TargetEntry{Target: target}); err != nil {
						t.Fatalf("StoreTarget() setup error: %v", err)
					}
				},
				wantStatus: linker.StatusLinked,
			},
			{
				name:       "missing",
				setup:      func(t *testing.T, root, source, target string) { t.Helper() },
				wantStatus: linker.StatusMissing,
			},
			{
				name: "conflict",
				setup: func(t *testing.T, root, source, target string) {
					t.Helper()
					if err := os.MkdirAll(target, 0o755); err != nil {
						t.Fatalf("MkdirAll(%q): %v", target, err)
					}
				},
				wantStatus: linker.StatusConflict,
			},
			{
				name: "broken",
				setup: func(t *testing.T, root, source, target string) {
					t.Helper()
					mustSymlink(t, source, target)
					if err := os.RemoveAll(source); err != nil {
						t.Fatalf("RemoveAll(%q): %v", source, err)
					}
				},
				wantStatus: linker.StatusBroken,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				root := t.TempDir()
				source := createStore(t, root, "app", map[string]string{"config.txt": "data"})
				target := filepath.Join(root, "targets", tt.name)
				tt.setup(t, root, source, target)

				results := GetStatus(root, "app", config.StoreEntry{Target: target})
				if len(results) != 1 {
					t.Fatalf("len(GetStatus()) = %d, want 1", len(results))
				}
				if results[0].Error != nil {
					t.Fatalf("GetStatus() returned unexpected error: %v", results[0].Error)
				}
				if results[0].Status != tt.wantStatus {
					t.Fatalf("status = %v, want %v", results[0].Status, tt.wantStatus)
				}
			})
		}
	})

	t.Run("file mode returns per file statuses", func(t *testing.T) {
		root := t.TempDir()
		te := config.TargetEntry{
			Target: filepath.Join(root, "targets", "files"),
			Files:  []string{"linked.txt", "conflict.txt", "broken.txt", "missing.txt"},
		}
		createStore(t, root, "app", map[string]string{
			"linked.txt":   "linked",
			"conflict.txt": "conflict",
			"broken.txt":   "broken",
			"missing.txt":  "missing",
		})

		mustSymlink(t, filepath.Join(root, "app", "linked.txt"), filepath.Join(te.Target, "linked.txt"))
		writeTestFile(t, filepath.Join(te.Target, "conflict.txt"), "target-conflict")
		mustSymlink(t, filepath.Join(root, "nowhere", "missing.txt"), filepath.Join(te.Target, "broken.txt"))

		results := GetStatus(root, "app", config.StoreEntry{Target: te.Target, Files: te.Files})
		if len(results) != 4 {
			t.Fatalf("len(GetStatus()) = %d, want 4", len(results))
		}

		got := make(map[string]StatusInfo, len(results))
		for _, info := range results {
			got[info.File] = info
		}

		want := map[string]linker.Status{
			"linked.txt":   linker.StatusLinked,
			"conflict.txt": linker.StatusConflict,
			"broken.txt":   linker.StatusBroken,
			"missing.txt":  linker.StatusMissing,
		}

		for file, wantStatus := range want {
			info, ok := got[file]
			if !ok {
				t.Fatalf("missing status for %q", file)
			}
			if info.Error != nil {
				t.Fatalf("status for %q returned unexpected error: %v", file, info.Error)
			}
			if info.Status != wantStatus {
				t.Fatalf("status for %q = %v, want %v", file, info.Status, wantStatus)
			}
		}
	})

	t.Run("auto promoted whole directory returns per file statuses", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "targets", "auto-status")
		createStore(t, root, "app", map[string]string{
			"keep.txt":           "keep",
			".store/config.yaml": "ignored",
		})

		if err := StoreTarget(root, "app", config.TargetEntry{Target: target}); err != nil {
			t.Fatalf("StoreTarget() setup error: %v", err)
		}

		results := GetStatus(root, "app", config.StoreEntry{Target: target})
		if len(results) != 1 {
			t.Fatalf("len(GetStatus()) = %d, want 1", len(results))
		}
		if results[0].Error != nil {
			t.Fatalf("GetStatus() returned unexpected error: %v", results[0].Error)
		}
		if results[0].File != "keep.txt" {
			t.Fatalf("file = %q, want keep.txt", results[0].File)
		}
		if results[0].Status != linker.StatusLinked {
			t.Fatalf("status = %v, want %v", results[0].Status, linker.StatusLinked)
		}
	})

	t.Run("no target configured returns error info", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{"config.txt": "data"})

		results := GetStatus(root, "app", config.StoreEntry{})
		if len(results) != 1 {
			t.Fatalf("len(GetStatus()) = %d, want 1", len(results))
		}
		if results[0].Error == nil || !strings.Contains(results[0].Error.Error(), "no target configured") {
			t.Fatalf("error = %v, want no target configured", results[0].Error)
		}
	})
}

func TestGetStatusAll(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Stores: map[string]config.StoreEntry{
		"app":    {Target: filepath.Join(root, "targets", "app")},
		"shells": {Target: filepath.Join(root, "targets", "home"), Files: []string{".zshrc"}},
	}}

	createStore(t, root, "app", map[string]string{"config.txt": "data"})
	createStore(t, root, "shells", map[string]string{".zshrc": "zsh"})

	if err := StoreTarget(root, "app", config.TargetEntry{Target: cfg.Stores["app"].Target}); err != nil {
		t.Fatalf("StoreTarget() setup error: %v", err)
	}

	results := GetStatusAll(root, cfg)
	if len(results) != 2 {
		t.Fatalf("len(GetStatusAll()) = %d, want 2", len(results))
	}

	got := statusMap(results)
	appInfo, ok := got[fmt.Sprintf("%s||%s", "app", cfg.Stores["app"].Target)]
	if !ok {
		t.Fatalf("missing app status")
	}
	if appInfo.Status != linker.StatusLinked || appInfo.Error != nil {
		t.Fatalf("app status = %+v, want linked with no error", appInfo)
	}

	shellsTarget := filepath.Join(cfg.Stores["shells"].Target, ".zshrc")
	shellsInfo, ok := got[fmt.Sprintf("%s|%s|%s", "shells", ".zshrc", shellsTarget)]
	if !ok {
		t.Fatalf("missing shells status")
	}
	if shellsInfo.Status != linker.StatusMissing || shellsInfo.Error != nil {
		t.Fatalf("shells status = %+v, want missing with no error", shellsInfo)
	}
}
