package linker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{name: "linked", status: StatusLinked, want: "linked"},
		{name: "missing", status: StatusMissing, want: "missing"},
		{name: "conflict", status: StatusConflict, want: "conflict"},
		{name: "broken", status: StatusBroken, want: "broken"},
		{name: "drift", status: StatusDrift, want: "drift"},
		{name: "unknown", status: Status(99), want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Fatalf("Status(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) (source, target string)
		want  Status
	}{
		{
			name: "missing target returns missing",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				return source, target
			},
			want: StatusMissing,
		},
		{
			name: "symlink to source returns linked",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				mustSymlink(t, source, target)
				return source, target
			},
			want: StatusLinked,
		},
		{
			name: "broken symlink to nonexistent path returns broken",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				brokenDest := filepath.Join(root, "missing-dest")
				mustSymlink(t, brokenDest, target)
				return source, target
			},
			want: StatusBroken,
		},
		{
			name: "regular file target returns conflict",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := mustWriteFile(t, filepath.Join(root, "target"), "conflict")
				return source, target
			},
			want: StatusConflict,
		},
		{
			name: "symlink to different existing path returns conflict",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				other := mustWriteFile(t, filepath.Join(root, "other.txt"), "other")
				target := filepath.Join(root, "target")
				mustSymlink(t, other, target)
				return source, target
			},
			want: StatusConflict,
		},
		{
			name: "missing source with symlink to it returns broken",
			setup: func(t *testing.T, root string) (string, string) {
				source := filepath.Join(root, "missing-source.txt")
				target := filepath.Join(root, "target")
				mustSymlink(t, source, target)
				return source, target
			},
			want: StatusBroken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source, target := tt.setup(t, root)

			got, err := Check(source, target)
			if err != nil {
				t.Fatalf("Check(%q, %q) returned error: %v", source, target, err)
			}
			if got != tt.want {
				t.Fatalf("Check(%q, %q) = %v, want %v", source, target, got, tt.want)
			}
		})
	}
}

func TestLink(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, root string) (source, target, wantReadlink string)
		wantErr     bool
		errContains string
		after       func(t *testing.T, root, source, target string)
	}{
		{
			name: "creates symlink when target missing",
			setup: func(t *testing.T, root string) (string, string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				return source, target, source
			},
		},
		{
			name: "no-op when already linked",
			setup: func(t *testing.T, root string) (string, string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				mustSymlink(t, source, target)
				return source, target, source
			},
		},
		{
			name: "returns error on regular file conflict",
			setup: func(t *testing.T, root string) (string, string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := mustWriteFile(t, filepath.Join(root, "target"), "conflict")
				return source, target, ""
			},
			wantErr:     true,
			errContains: "conflict",
			after: func(t *testing.T, root, source, target string) {
				fi, err := os.Lstat(target)
				if err != nil {
					t.Fatalf("Lstat(%q) after Link conflict: %v", target, err)
				}
				if fi.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("target %q became a symlink on conflict", target)
				}
			},
		},
		{
			name: "replaces broken symlink",
			setup: func(t *testing.T, root string) (string, string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				mustSymlink(t, filepath.Join(root, "missing-dest"), target)
				return source, target, source
			},
		},
		{
			name: "creates parent directories when needed",
			setup: func(t *testing.T, root string) (string, string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "nested", "deeper", "target")
				return source, target, source
			},
			after: func(t *testing.T, root, source, target string) {
				fi, err := os.Stat(filepath.Dir(target))
				if err != nil {
					t.Fatalf("Stat(%q): %v", filepath.Dir(target), err)
				}
				if !fi.IsDir() {
					t.Fatalf("parent %q is not a directory", filepath.Dir(target))
				}
			},
		},
		{
			name: "uses absolute path for symlink source",
			setup: func(t *testing.T, root string) (string, string, string) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("Getwd(): %v", err)
				}
				if err := os.Chdir(root); err != nil {
					t.Fatalf("Chdir(%q): %v", root, err)
				}
				t.Cleanup(func() {
					if err := os.Chdir(cwd); err != nil {
						t.Fatalf("restore working directory: %v", err)
					}
				})

				sourceAbs := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				targetAbs := filepath.Join(root, "links", "target")
				sourceRel, err := filepath.Rel(root, sourceAbs)
				if err != nil {
					t.Fatalf("Rel(source): %v", err)
				}
				targetRel, err := filepath.Rel(root, targetAbs)
				if err != nil {
					t.Fatalf("Rel(target): %v", err)
				}

				return sourceRel, targetRel, sourceAbs
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source, target, wantReadlink := tt.setup(t, root)

			err := Link(source, target)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Link(%q, %q) error = nil, want error", source, target)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("Link(%q, %q) error = %q, want substring %q", source, target, err.Error(), tt.errContains)
				}
				if tt.after != nil {
					tt.after(t, root, source, target)
				}
				return
			}

			if err != nil {
				t.Fatalf("Link(%q, %q) returned error: %v", source, target, err)
			}

			gotReadlink, err := os.Readlink(target)
			if err != nil {
				t.Fatalf("Readlink(%q): %v", target, err)
			}
			// Normalize via EvalSymlinks so macOS canonicalization
			// (/private/var/...) and Windows 8.3 short names don't cause
			// cosmetic mismatches — Link and the expected path may come
			// from different resolution paths.
			gotResolved, err := filepath.EvalSymlinks(gotReadlink)
			if err != nil {
				t.Fatalf("EvalSymlinks(%q) error = %v", gotReadlink, err)
			}
			wantResolved, err := filepath.EvalSymlinks(wantReadlink)
			if err != nil {
				t.Fatalf("EvalSymlinks(%q) error = %v", wantReadlink, err)
			}
			if gotResolved != wantResolved {
				t.Fatalf("Readlink(%q) = %q, want %q", target, gotReadlink, wantReadlink)
			}

			if tt.after != nil {
				tt.after(t, root, source, target)
			}
		})
	}
}

func TestUnlink(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, root string) (source, target string)
		wantErr     bool
		errContains string
		after       func(t *testing.T, root, source, target string)
	}{
		{
			name: "removes valid symlink",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "target")
				mustSymlink(t, source, target)
				return source, target
			},
		},
		{
			name: "no-op when target missing",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := filepath.Join(root, "missing-target")
				return source, target
			},
		},
		{
			name: "returns error when target is not our symlink",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "source")
				target := mustWriteFile(t, filepath.Join(root, "target"), "conflict")
				return source, target
			},
			wantErr:     true,
			errContains: "refusing to remove",
			after: func(t *testing.T, root, source, target string) {
				fi, err := os.Lstat(target)
				if err != nil {
					t.Fatalf("Lstat(%q) after Unlink conflict: %v", target, err)
				}
				if fi.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("target %q unexpectedly became a symlink", target)
				}
			},
		},
		{
			name: "removes broken symlink",
			setup: func(t *testing.T, root string) (string, string) {
				source := filepath.Join(root, "missing-source.txt")
				target := filepath.Join(root, "target")
				mustSymlink(t, source, target)
				return source, target
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source, target := tt.setup(t, root)

			err := Unlink(source, target)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unlink(%q, %q) error = nil, want error", source, target)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("Unlink(%q, %q) error = %q, want substring %q", source, target, err.Error(), tt.errContains)
				}
				if tt.after != nil {
					tt.after(t, root, source, target)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unlink(%q, %q) returned error: %v", source, target, err)
			}

			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				if err != nil {
					t.Fatalf("Lstat(%q) after Unlink = %v, want not exists", target, err)
				}
				t.Fatalf("target %q still exists after Unlink", target)
			}

			if tt.after != nil {
				tt.after(t, root, source, target)
			}
		})
	}
}

func TestCheckRendered(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) (source, target string)
		want  Status
	}{
		{
			name: "target missing returns missing",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "hello")
				target := filepath.Join(root, "target.txt")
				return source, target
			},
			want: StatusMissing,
		},
		{
			name: "target is a symlink returns conflict",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "hello")
				other := mustWriteFile(t, filepath.Join(root, "other.txt"), "hello")
				target := filepath.Join(root, "target.txt")
				mustSymlink(t, other, target)
				return source, target
			},
			want: StatusConflict,
		},
		{
			name: "target is a directory returns conflict",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "hello")
				target := filepath.Join(root, "targetdir")
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatalf("MkdirAll(%q): %v", target, err)
				}
				return source, target
			},
			want: StatusConflict,
		},
		{
			name: "target content matches source returns linked",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "hello")
				target := mustWriteFile(t, filepath.Join(root, "target.txt"), "hello")
				return source, target
			},
			want: StatusLinked,
		},
		{
			name: "target content differs from source returns drift",
			setup: func(t *testing.T, root string) (string, string) {
				source := mustWriteFile(t, filepath.Join(root, "source.txt"), "hello")
				target := mustWriteFile(t, filepath.Join(root, "target.txt"), "world")
				return source, target
			},
			want: StatusDrift,
		},
		{
			name: "source missing returns broken",
			setup: func(t *testing.T, root string) (string, string) {
				source := filepath.Join(root, "missing-source.txt")
				target := mustWriteFile(t, filepath.Join(root, "target.txt"), "hello")
				return source, target
			},
			want: StatusBroken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source, target := tt.setup(t, root)

			got, err := CheckRendered(source, target)
			if err != nil {
				t.Fatalf("CheckRendered(%q, %q) returned error: %v", source, target, err)
			}
			if got != tt.want {
				t.Fatalf("CheckRendered(%q, %q) = %v, want %v", source, target, got, tt.want)
			}
		})
	}
}

func mustWriteFile(t *testing.T, path, contents string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}

	return path
}

func mustSymlink(t *testing.T, source, target string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(target), err)
	}
	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("Symlink(%q, %q): %v", source, target, err)
	}
}
