package store

import (
	"path/filepath"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
)

func TestCollectTargetConflicts(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, root string) config.TargetEntry
		wantCount int
		check     func(t *testing.T, root string, conflicts []ConflictInfo)
	}{
		{
			name: "whole directory conflict with regular dir",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				target := filepath.Join(root, "targets", "app")
				writeTestFile(t, filepath.Join(target, "existing.txt"), "existing")
				return config.TargetEntry{Target: target}
			},
			wantCount: 1,
			check: func(t *testing.T, root string, conflicts []ConflictInfo) {
				t.Helper()
				if conflicts[0].Source != filepath.Join(root, "app") {
					t.Fatalf("Source = %q, want %q", conflicts[0].Source, filepath.Join(root, "app"))
				}
				if !conflicts[0].IsDir {
					t.Fatalf("IsDir = false, want true")
				}
			},
		},
		{
			name: "file mode conflicts with regular files",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"a.txt":        "a",
					"nested/b.txt": "b",
				})
				target := filepath.Join(root, "targets", "files")
				writeTestFile(t, filepath.Join(target, "a.txt"), "target-a")
				writeTestFile(t, filepath.Join(target, "nested", "b.txt"), "target-b")
				return config.TargetEntry{Target: target, Files: []string{"a.txt", "nested/b.txt"}}
			},
			wantCount: 2,
			check: func(t *testing.T, root string, conflicts []ConflictInfo) {
				t.Helper()
				seen := map[string]ConflictInfo{}
				for _, c := range conflicts {
					seen[c.Target] = c
				}
				for _, rel := range []string{"a.txt", "nested/b.txt"} {
					target := filepath.Join(root, "targets", "files", rel)
					c, ok := seen[target]
					if !ok {
						t.Fatalf("missing conflict for %q", target)
					}
					if c.IsDir {
						t.Fatalf("IsDir for %q = true, want false", target)
					}
				}
			},
		},
		{
			name: "no conflicts when target missing",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{"a.txt": "a"})
				return config.TargetEntry{Target: filepath.Join(root, "targets", "missing"), Files: []string{"a.txt"}}
			},
			wantCount: 0,
		},
		{
			name: "no conflicts when already linked",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				te := config.TargetEntry{Target: filepath.Join(root, "targets", "app")}
				createStore(t, root, "app", map[string]string{"config.txt": "data"})
				if err := StoreTarget(root, "app", te); err != nil {
					t.Fatalf("StoreTarget() setup error: %v", err)
				}
				return te
			},
			wantCount: 0,
		},
		{
			name: "mix of conflicting and non conflicting file targets",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				te := config.TargetEntry{Target: filepath.Join(root, "targets", "files"), Files: []string{"conflict.txt", "linked.txt", "missing.txt"}}
				createStore(t, root, "app", map[string]string{
					"conflict.txt": "conflict",
					"linked.txt":   "linked",
					"missing.txt":  "missing",
				})
				writeTestFile(t, filepath.Join(te.Target, "conflict.txt"), "target-conflict")
				mustSymlink(t, filepath.Join(root, "app", "linked.txt"), filepath.Join(te.Target, "linked.txt"))
				return te
			},
			wantCount: 1,
			check: func(t *testing.T, root string, conflicts []ConflictInfo) {
				t.Helper()
				if conflicts[0].Target != filepath.Join(root, "targets", "files", "conflict.txt") {
					t.Fatalf("Target = %q, want conflict target", conflicts[0].Target)
				}
			},
		},
		{
			name: "whole directory auto promotion checks file conflicts",
			setup: func(t *testing.T, root string) config.TargetEntry {
				t.Helper()
				createStore(t, root, "app", map[string]string{
					"keep.txt":           "keep",
					".store/config.yaml": "ignored",
				})
				target := filepath.Join(root, "targets", "auto")
				writeTestFile(t, filepath.Join(target, "keep.txt"), "target-keep")
				return config.TargetEntry{Target: target}
			},
			wantCount: 1,
			check: func(t *testing.T, root string, conflicts []ConflictInfo) {
				t.Helper()
				if conflicts[0].Source != filepath.Join(root, "app", "keep.txt") {
					t.Fatalf("Source = %q, want keep.txt source", conflicts[0].Source)
				}
				if conflicts[0].Target != filepath.Join(root, "targets", "auto", "keep.txt") {
					t.Fatalf("Target = %q, want keep.txt target", conflicts[0].Target)
				}
				if conflicts[0].IsDir {
					t.Fatalf("IsDir = true, want false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			te := tt.setup(t, root)

			conflicts, err := CollectTargetConflicts(root, "app", te)
			if err != nil {
				t.Fatalf("CollectTargetConflicts() error = %v", err)
			}
			if len(conflicts) != tt.wantCount {
				t.Fatalf("len(conflicts) = %d, want %d", len(conflicts), tt.wantCount)
			}
			if tt.check != nil {
				tt.check(t, root, conflicts)
			}
		})
	}
}

func TestCollectConflicts(t *testing.T) {
	root := t.TempDir()
	createStore(t, root, "app", map[string]string{
		"a.txt":        "a",
		"nested/b.txt": "b",
	})

	wholeTarget := filepath.Join(root, "targets", "whole")
	fileTarget := filepath.Join(root, "targets", "files")
	writeTestFile(t, filepath.Join(wholeTarget, "existing.txt"), "existing")
	writeTestFile(t, filepath.Join(fileTarget, "nested", "b.txt"), "target-b")

	entry := config.StoreEntry{Targets: []config.TargetEntry{
		{Target: wholeTarget},
		{Target: fileTarget, Files: []string{"nested/b.txt"}},
	}}

	conflicts, err := CollectConflicts(root, "app", entry)
	if err != nil {
		t.Fatalf("CollectConflicts() error = %v", err)
	}
	if len(conflicts) != 2 {
		t.Fatalf("len(conflicts) = %d, want 2", len(conflicts))
	}
}

func TestResolveConflicts(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) []ConflictInfo
		check func(t *testing.T, root string)
	}{
		{
			name: "moves file from target to source",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				source := filepath.Join(root, "repo", "app", "config.txt")
				target := filepath.Join(root, "target", "config.txt")
				writeTestFile(t, target, "from-target")
				return []ConflictInfo{{Source: source, Target: target, IsDir: false}}
			},
			check: func(t *testing.T, root string) {
				t.Helper()
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt"), "from-target")
				assertMissing(t, filepath.Join(root, "target", "config.txt"))
			},
		},
		{
			name: "moves directory contents and merges",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				source := filepath.Join(root, "repo", "app")
				target := filepath.Join(root, "target", "app")
				writeTestFile(t, filepath.Join(source, "keep.txt"), "keep")
				writeTestFile(t, filepath.Join(target, "nested", "config.txt"), "config")
				writeTestFile(t, filepath.Join(target, "extra.txt"), "extra")
				return []ConflictInfo{{Source: source, Target: target, IsDir: true}}
			},
			check: func(t *testing.T, root string) {
				t.Helper()
				assertFileContent(t, filepath.Join(root, "repo", "app", "keep.txt"), "keep")
				assertFileContent(t, filepath.Join(root, "repo", "app", "nested", "config.txt"), "config")
				assertFileContent(t, filepath.Join(root, "repo", "app", "extra.txt"), "extra")
				assertMissing(t, filepath.Join(root, "target", "app"))
			},
		},
		{
			name: "backs up existing source files with bak suffix",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				source := filepath.Join(root, "repo", "app", "config.txt")
				target := filepath.Join(root, "target", "config.txt")
				writeTestFile(t, source, "from-source")
				writeTestFile(t, target, "from-target")
				return []ConflictInfo{{Source: source, Target: target, IsDir: false}}
			},
			check: func(t *testing.T, root string) {
				t.Helper()
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt"), "from-target")
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt.bak"), "from-source")
			},
		},
		{
			name: "uses nested bak numbering when needed",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				source := filepath.Join(root, "repo", "app", "config.txt")
				target := filepath.Join(root, "target", "config.txt")
				writeTestFile(t, source, "current-source")
				writeTestFile(t, source+".bak", "bak0")
				writeTestFile(t, source+".bak.1", "bak1")
				writeTestFile(t, target, "from-target")
				return []ConflictInfo{{Source: source, Target: target, IsDir: false}}
			},
			check: func(t *testing.T, root string) {
				t.Helper()
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt"), "from-target")
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt.bak"), "bak0")
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt.bak.1"), "bak1")
				assertFileContent(t, filepath.Join(root, "repo", "app", "config.txt.bak.2"), "current-source")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			conflicts := tt.setup(t, root)

			if err := ResolveConflicts(conflicts); err != nil {
				t.Fatalf("ResolveConflicts() error = %v", err)
			}
			tt.check(t, root)
		})
	}
}

func TestCollectBackups(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) []ConflictInfo
		check func(t *testing.T, backups []string)
	}{
		{
			name: "identifies file backups",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				source := filepath.Join(root, "repo", "app", "config.txt")
				writeTestFile(t, source, "source")
				return []ConflictInfo{{Source: source, Target: filepath.Join(root, "target", "config.txt"), IsDir: false}}
			},
			check: func(t *testing.T, backups []string) {
				t.Helper()
				if len(backups) != 1 {
					t.Fatalf("len(backups) = %d, want 1", len(backups))
				}
				if filepath.Base(backups[0]) != "config.txt" {
					t.Fatalf("backup = %q, want config.txt path", backups[0])
				}
			},
		},
		{
			name: "identifies directory content backups",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				source := filepath.Join(root, "repo", "app")
				target := filepath.Join(root, "target", "app")
				writeTestFile(t, filepath.Join(source, "nested", "keep.txt"), "source-keep")
				writeTestFile(t, filepath.Join(source, "overlap.txt"), "source-overlap")
				writeTestFile(t, filepath.Join(target, "nested", "keep.txt"), "target-keep")
				writeTestFile(t, filepath.Join(target, "overlap.txt"), "target-overlap")
				writeTestFile(t, filepath.Join(target, "new.txt"), "new")
				return []ConflictInfo{{Source: source, Target: target, IsDir: true}}
			},
			check: func(t *testing.T, backups []string) {
				t.Helper()
				if len(backups) != 2 {
					t.Fatalf("len(backups) = %d, want 2", len(backups))
				}
				seen := map[string]bool{}
				for _, backup := range backups {
					seen[filepath.Base(backup)] = true
				}
				if !seen["keep.txt"] || !seen["overlap.txt"] {
					t.Fatalf("backups = %v, want keep.txt and overlap.txt", backups)
				}
			},
		},
		{
			name: "returns empty when no backups are needed",
			setup: func(t *testing.T, root string) []ConflictInfo {
				t.Helper()
				return []ConflictInfo{{
					Source: filepath.Join(root, "repo", "app", "config.txt"),
					Target: filepath.Join(root, "target", "config.txt"),
					IsDir:  false,
				}}
			},
			check: func(t *testing.T, backups []string) {
				t.Helper()
				if len(backups) != 0 {
					t.Fatalf("len(backups) = %d, want 0", len(backups))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			conflicts := tt.setup(t, root)

			backups, err := CollectBackups(conflicts)
			if err != nil {
				t.Fatalf("CollectBackups() error = %v", err)
			}
			tt.check(t, backups)
		})
	}
}
