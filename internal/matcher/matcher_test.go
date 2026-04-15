package matcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		files    []string
		dirs     []string
		inputs   []string
		patterns []string
		ignore   []string
		want     []string
		wantErr  string
	}{
		{
			name:   "explicit single file",
			files:  []string{"one.txt"},
			inputs: []string{"one.txt"},
			want:   []string{"one.txt"},
		},
		{
			name:   "explicit multiple files",
			files:  []string{"b.txt", "a.txt"},
			inputs: []string{"b.txt", "a.txt"},
			want:   []string{"a.txt", "b.txt"},
		},
		{
			name:   "explicit nested file",
			files:  []string{"dir/file.txt"},
			inputs: []string{"dir/file.txt"},
			want:   []string{"dir/file.txt"},
		},
		{
			name:   "explicit empty string skipped",
			files:  []string{"keep.txt"},
			inputs: []string{"", "keep.txt"},
			want:   []string{"keep.txt"},
		},
		{
			name:    "explicit file not found",
			inputs:  []string{"missing.txt"},
			wantErr: "file \"missing.txt\" not found in store directory",
		},
		{
			name:    "explicit file path traversal rejected",
			inputs:  []string{"../escape"},
			wantErr: "file path \"../escape\" escapes store directory",
		},
		{
			name:    "explicit dot path rejected",
			inputs:  []string{"."},
			wantErr: "file path \".\" escapes store directory",
		},
		{
			name:     "simple txt glob",
			files:    []string{"match.txt", "other.lua", "nested/skip.txt"},
			patterns: []string{"*.txt"},
			want:     []string{"match.txt"},
		},
		{
			name:     "simple lua glob",
			files:    []string{"top.lua", "nested/deep.lua", "readme.md"},
			patterns: []string{"*.lua"},
			want:     []string{"top.lua"},
		},
		{
			name:     "simple glob matching nothing",
			files:    []string{"one.txt", "two.lua"},
			patterns: []string{"*.conf"},
			want:     []string{},
		},
		{
			name:     "recursive conf glob",
			files:    []string{"root.conf", "dir/app.conf", "dir/nested/db.conf", "dir/nested/skip.txt"},
			patterns: []string{"**/*.conf"},
			want:     []string{"dir/app.conf", "dir/nested/db.conf", "root.conf"},
		},
		{
			name:     "recursive lua glob",
			files:    []string{"init.lua", "lua/plugins/a.lua", "lua/plugins/deep/b.lua", "notes.txt"},
			patterns: []string{"**/*.lua"},
			want:     []string{"init.lua", "lua/plugins/a.lua", "lua/plugins/deep/b.lua"},
		},
		{
			name:     "combined files and patterns are deduplicated",
			files:    []string{"alpha.txt", "beta.txt", "gamma.lua"},
			inputs:   []string{"alpha.txt"},
			patterns: []string{"*.txt"},
			want:     []string{"alpha.txt", "beta.txt"},
		},
		{
			name:     "directories excluded from glob results",
			files:    []string{"file.txt", "dir/nested.txt"},
			dirs:     []string{"matched-dir"},
			patterns: []string{"*"},
			want:     []string{"file.txt"},
		},
		{
			name: "empty inputs return empty slice",
			want: []string{},
		},
		{
			name:     "results are sorted alphabetically",
			files:    []string{"z.txt", "a.txt", "dir/c.txt", "b.lua"},
			inputs:   []string{"z.txt", "a.txt", "dir/c.txt"},
			patterns: []string{"*.lua"},
			want:     []string{"a.txt", "b.lua", "dir/c.txt", "z.txt"},
		},
		{
			name:     "empty pattern skipped",
			files:    []string{"keep.txt", "other.lua"},
			patterns: []string{"", "*.txt"},
			want:     []string{"keep.txt"},
		},
		{
			name:     "pattern path traversal rejected",
			files:    []string{"keep.txt"},
			patterns: []string{"../*"},
			wantErr:  "pattern \"../*\" escapes store directory",
		},
		{
			name:   "explicit file matching global ignore is excluded",
			files:  []string{".store/config.yaml", "keep.txt"},
			inputs: []string{".store/config.yaml", "keep.txt"},
			want:   []string{"keep.txt"},
		},
		{
			name:     "glob excludes global ignore matches",
			files:    []string{"keep.txt", "nested/keep.txt", ".store/config.yaml", ".git/config", ".DS_Store"},
			patterns: []string{"**/*"},
			want:     []string{"keep.txt", "nested/keep.txt"},
		},
		{
			name:     "user ignore excludes directory contents",
			files:    []string{"scratch/notes.txt", "keep.txt"},
			patterns: []string{"**/*"},
			ignore:   []string{"scratch/"},
			want:     []string{"keep.txt"},
		},
		{
			name:     "user and global ignore both apply",
			files:    []string{"scratch/notes.txt", ".git/config", "keep.txt"},
			patterns: []string{"**/*"},
			ignore:   []string{"scratch/"},
			want:     []string{"keep.txt"},
		},
		{
			name:     "empty ignore list keeps non global files",
			files:    []string{"keep.txt", "nested/keep.txt", ".DS_Store"},
			patterns: []string{"**/*"},
			ignore:   []string{},
			want:     []string{"keep.txt", "nested/keep.txt"},
		},
		{
			name:     "ignore doublestar pattern excludes matching files",
			files:    []string{"keep.txt", "nested/backup.bak", "deep/also.bak"},
			patterns: []string{"**/*"},
			ignore:   []string{"**/*.bak"},
			want:     []string{"keep.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storeDir := buildTempStoreDir(t, tt.files, tt.dirs)

			got, err := Match(storeDir, tt.inputs, tt.patterns, tt.ignore)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Match() error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("Match() error = %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Match() unexpected error = %v", err)
			}

			assertPathsEqual(t, got, tt.want)
		})
	}
}

func buildTempStoreDir(t *testing.T, files []string, dirs []string) string {
	t.Helper()

	storeDir := t.TempDir()

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(storeDir, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) failed: %v", dir, err)
		}
	}

	for _, file := range files {
		fullPath := filepath.Join(storeDir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(file), err)
		}
		if err := os.WriteFile(fullPath, []byte(file), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) failed: %v", file, err)
		}
	}

	return storeDir
}

func assertPathsEqual(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("Match() returned %v, want %v", got, want)
	}

	// Match returns paths using the OS-native separator, but test data is
	// written with forward slashes for readability. Normalize both sides to
	// forward slashes before comparing so the same table works on Windows.
	for i := range want {
		if filepath.ToSlash(got[i]) != filepath.ToSlash(want[i]) {
			t.Fatalf("Match() returned %v, want %v", got, want)
		}
	}
}
