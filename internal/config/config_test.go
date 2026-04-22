package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTargetEntryHasFileMode(t *testing.T) {
	tests := []struct {
		name  string
		entry TargetEntry
		want  bool
	}{
		{
			name:  "files only",
			entry: TargetEntry{Files: []string{"a.txt"}},
			want:  true,
		},
		{
			name:  "patterns only",
			entry: TargetEntry{Patterns: []string{"*.txt"}},
			want:  true,
		},
		{
			name:  "files and patterns",
			entry: TargetEntry{Files: []string{"a.txt"}, Patterns: []string{"*.txt"}},
			want:  true,
		},
		{
			name:  "neither",
			entry: TargetEntry{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.HasFileMode(); got != tt.want {
				t.Fatalf("HasFileMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreEntryHasFileMode(t *testing.T) {
	tests := []struct {
		name  string
		entry StoreEntry
		want  bool
	}{
		{
			name:  "single target directory mode",
			entry: StoreEntry{Target: "~/.config/nvim"},
			want:  false,
		},
		{
			name:  "single target file mode",
			entry: StoreEntry{Target: "~", Files: []string{".zshrc"}},
			want:  true,
		},
		{
			name: "multi target all directory mode",
			entry: StoreEntry{Targets: []TargetEntry{
				{Target: "~/.config/nvim"},
				{Target: "~/.config/git"},
			}},
			want: false,
		},
		{
			name: "multi target mixed modes",
			entry: StoreEntry{Targets: []TargetEntry{
				{Target: "~/.config/nvim"},
				{Target: "~/.config/fish", Patterns: []string{"*.fish"}},
			}},
			want: true,
		},
		{
			name:  "no targets",
			entry: StoreEntry{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.HasFileMode(); got != tt.want {
				t.Fatalf("HasFileMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreEntryIsMultiTarget(t *testing.T) {
	tests := []struct {
		name  string
		entry StoreEntry
		want  bool
	}{
		{
			name:  "single target field only",
			entry: StoreEntry{Target: "~/.config/nvim"},
			want:  false,
		},
		{
			name:  "nil targets",
			entry: StoreEntry{Targets: nil},
			want:  false,
		},
		{
			name:  "empty targets slice",
			entry: StoreEntry{Targets: []TargetEntry{}},
			want:  false,
		},
		{
			name:  "targets present",
			entry: StoreEntry{Targets: []TargetEntry{{Target: "~/.config/nvim"}}},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.IsMultiTarget(); got != tt.want {
				t.Fatalf("IsMultiTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreEntryResolvedTargets(t *testing.T) {
	multiTargets := []TargetEntry{
		{Target: "~", Files: []string{".zshrc"}},
		{Target: "~/.config/fish", Patterns: []string{"*.fish"}},
	}

	tests := []struct {
		name  string
		entry StoreEntry
		want  []TargetEntry
	}{
		{
			name: "single target",
			entry: StoreEntry{
				Target:   "~",
				Files:    []string{".zshrc"},
				Patterns: []string{".bash*"},
				Ignore:   []string{"*.bak"},
			},
			want: []TargetEntry{{
				Target:   "~",
				Files:    []string{".zshrc"},
				Patterns: []string{".bash*"},
				Ignore:   []string{"*.bak"},
			}},
		},
		{
			name:  "multi target",
			entry: StoreEntry{Targets: multiTargets},
			want:  multiTargets,
		},
		{
			name:  "no target",
			entry: StoreEntry{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.ResolvedTargets(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolvedTargets() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStoreEntryValidate(t *testing.T) {
	tests := []struct {
		name        string
		entry       StoreEntry
		wantErr     string
		wantWarning string
	}{
		{
			name:  "valid single target",
			entry: StoreEntry{Target: "~/.config/nvim"},
		},
		{
			name: "valid multi target",
			entry: StoreEntry{Targets: []TargetEntry{
				{Target: "~", Files: []string{".zshrc"}},
				{Target: "~/.config/fish", Patterns: []string{"*.fish"}},
			}},
		},
		{
			name:    "target and targets both set",
			entry:   StoreEntry{Target: "~", Targets: []TargetEntry{{Target: "~/.config/fish"}}},
			wantErr: "cannot use both 'target' and 'targets' on the same store entry",
		},
		{
			name:    "top level files with targets",
			entry:   StoreEntry{Files: []string{".zshrc"}, Targets: []TargetEntry{{Target: "~"}}},
			wantErr: "cannot use top-level 'files', 'patterns', or 'ignore' with 'targets'; place them inside each target entry",
		},
		{
			name:    "top level patterns with targets",
			entry:   StoreEntry{Patterns: []string{"*.fish"}, Targets: []TargetEntry{{Target: "~/.config/fish"}}},
			wantErr: "cannot use top-level 'files', 'patterns', or 'ignore' with 'targets'; place them inside each target entry",
		},
		{
			name:    "top level ignore with targets",
			entry:   StoreEntry{Ignore: []string{"*.bak"}, Targets: []TargetEntry{{Target: "~/.config/fish"}}},
			wantErr: "cannot use top-level 'files', 'patterns', or 'ignore' with 'targets'; place them inside each target entry",
		},
		{
			name:    "missing target path in targets list",
			entry:   StoreEntry{Targets: []TargetEntry{{Target: "~"}, {}}},
			wantErr: "targets[1]: target path is required",
		},
		{
			name:        "empty entry warning",
			entry:       StoreEntry{},
			wantWarning: "warning: store entry has no target configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning, err := captureStderr(func() error {
				return tt.entry.Validate()
			})

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("Validate() error = nil, want substring %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}

			if tt.wantWarning == "" {
				if warning != "" {
					t.Fatalf("Validate() warning = %q, want none", warning)
				}
			} else if !strings.Contains(warning, tt.wantWarning) {
				t.Fatalf("Validate() warning = %q, want substring %q", warning, tt.wantWarning)
			}
		})
	}
}

func TestStoreEntryMigrateToMultiTarget(t *testing.T) {
	tests := []struct {
		name string
		in   StoreEntry
		want StoreEntry
	}{
		{
			name: "migrates single target and clears original fields",
			in: StoreEntry{
				Target:   "~",
				Files:    []string{".zshrc"},
				Patterns: []string{".bash*"},
				Ignore:   []string{"*.bak"},
				Hooks:    &HookEntry{Post: "echo done"},
			},
			want: StoreEntry{
				Targets: []TargetEntry{{
					Target:   "~",
					Files:    []string{".zshrc"},
					Patterns: []string{".bash*"},
					Ignore:   []string{"*.bak"},
				}},
				Hooks: &HookEntry{Post: "echo done"},
			},
		},
		{
			name: "no op when target empty",
			in: StoreEntry{
				Files:    []string{".zshrc"},
				Patterns: []string{".bash*"},
				Targets:  []TargetEntry{{Target: "~/.config/fish"}},
			},
			want: StoreEntry{
				Files:    []string{".zshrc"},
				Patterns: []string{".bash*"},
				Targets:  []TargetEntry{{Target: "~/.config/fish"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in
			got.MigrateToMultiTarget()

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MigrateToMultiTarget() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStoreEntryMigrateToSingleTarget(t *testing.T) {
	tests := []struct {
		name string
		in   StoreEntry
		want StoreEntry
	}{
		{
			name: "single target collapses",
			in: StoreEntry{
				Targets: []TargetEntry{{
					Target:   "~",
					Files:    []string{".zshrc"},
					Patterns: []string{".bash*"},
					Ignore:   []string{"*.bak"},
				}},
				Hooks: &HookEntry{Pre: "echo pre"},
			},
			want: StoreEntry{
				Target:   "~",
				Files:    []string{".zshrc"},
				Patterns: []string{".bash*"},
				Ignore:   []string{"*.bak"},
				Hooks:    &HookEntry{Pre: "echo pre"},
			},
		},
		{
			name: "multiple targets stays multi target",
			in: StoreEntry{
				Targets: []TargetEntry{{Target: "~"}, {Target: "~/.config/fish"}},
			},
			want: StoreEntry{
				Targets: []TargetEntry{{Target: "~"}, {Target: "~/.config/fish"}},
			},
		},
		{
			name: "zero targets no op",
			in: StoreEntry{
				Target:   "~/.config/nvim",
				Files:    []string{},
				Patterns: nil,
			},
			want: StoreEntry{
				Target:   "~/.config/nvim",
				Files:    []string{},
				Patterns: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in
			got.MigrateToSingleTarget()

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("MigrateToSingleTarget() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestConfigLoadSaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := &Config{
		Stores: map[string]StoreEntry{
			"git": {
				Target: "~/.config/git",
				Ignore: []string{"*.bak"},
				Hooks:  &HookEntry{Post: "git config --global include.path ~/.config/git/config"},
				When:   &WhenClause{OS: Strings{"linux"}, Shell: Strings{"zsh"}},
			},
			"shells": {
				Targets: []TargetEntry{
					{Target: "~", Files: []string{".zshrc", ".bashrc"}, Ignore: []string{"*.bak"}},
					{Target: "~/.config/fish", Patterns: []string{"*.fish"}, Ignore: []string{"scratch/"}},
				},
			},
		},
	}

	if err := Save(root, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip config = %#v, want %#v", got, want)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, root string)
		wantErr     string
		checkErrIs  error
		checkConfig func(t *testing.T, cfg *Config)
	}{
		{
			name:       "missing file",
			setup:      func(t *testing.T, root string) {},
			wantErr:    "failed to read config",
			checkErrIs: os.ErrNotExist,
		},
		{
			name: "invalid yaml",
			setup: func(t *testing.T, root string) {
				writeConfigFile(t, root, "stores: [")
			},
			wantErr: "failed to parse config",
		},
		{
			name: "validation error",
			setup: func(t *testing.T, root string) {
				writeConfigFile(t, root, "stores:\n  shells:\n    target: \"~\"\n    targets:\n      - target: \"~/.config/fish\"\n")
			},
			wantErr: "store \"shells\": cannot use both 'target' and 'targets' on the same store entry",
		},
		{
			name: "missing stores map becomes empty map",
			setup: func(t *testing.T, root string) {
				writeConfigFile(t, root, "{}\n")
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				if cfg == nil {
					t.Fatal("Load() returned nil config")
				}
				if cfg.Stores == nil {
					t.Fatal("Load() left Stores map nil")
				}
				if len(cfg.Stores) != 0 {
					t.Fatalf("Load() Stores length = %d, want 0", len(cfg.Stores))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			cfg, err := Load(root)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Load() unexpected error: %v", err)
				}
				if tt.checkConfig != nil {
					tt.checkConfig(t, cfg)
				}
				return
			}

			if err == nil {
				t.Fatalf("Load() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
			if tt.checkErrIs != nil && !errors.Is(err, tt.checkErrIs) {
				t.Fatalf("Load() error = %v, want errors.Is(..., %v)", err, tt.checkErrIs)
			}
		})
	}
}

func TestSaveCreatesDirectoryIfMissing(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{Stores: map[string]StoreEntry{"nvim": {Target: "~/.config/nvim"}}}

	if err := Save(root, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(ConfigPath(root)); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ConfigDir)); err != nil {
		t.Fatalf("config directory was not created: %v", err)
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  bool
	}{
		{
			name:  "missing config",
			setup: func(t *testing.T, root string) {},
			want:  false,
		},
		{
			name: "config exists",
			setup: func(t *testing.T, root string) {
				writeConfigFile(t, root, "stores: {}\n")
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			if got := Exists(root); got != tt.want {
				t.Fatalf("Exists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty string", path: "", want: ""},
		{name: "tilde only", path: "~", want: home},
		{name: "tilde slash path", path: "~/foo", want: filepath.Join(home, "foo")},
		{name: "absolute path", path: "/absolute/path", want: "/absolute/path"},
		{name: "tilde username style", path: "~notuser", want: "~notuser"},
		{name: "path without tilde", path: "relative/path", want: "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandHome(tt.path)
			if err != nil {
				t.Fatalf("ExpandHome() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ExpandHome() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindRoot(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr string
		want    func(base string) string
	}{
		{
			name: "finds store in current directory",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				mustMkdirAll(t, filepath.Join(root, ConfigDir))
				return root
			},
			want: func(base string) string { return base },
		},
		{
			name: "finds store in parent directory",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				mustMkdirAll(t, filepath.Join(root, ConfigDir))
				child := filepath.Join(root, "a", "b")
				mustMkdirAll(t, child)
				return child
			},
			want: func(base string) string {
				return filepath.Clean(filepath.Join(base, "..", ".."))
			},
		},
		{
			name: "returns error when none found",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: "no .store directory found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := tt.setup(t)
			if err := os.Chdir(cwd); err != nil {
				t.Fatalf("os.Chdir(%q) error = %v", cwd, err)
			}
			defer func() {
				if err := os.Chdir(originalWD); err != nil {
					t.Fatalf("restore os.Chdir(%q) error = %v", originalWD, err)
				}
			}()

			got, err := FindRoot()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("FindRoot() error = %v", err)
				}
				// macOS canonicalizes /var/folders → /private/var/folders on
				// Chdir, and Windows returns 8.3 short names from Getwd.
				// Normalize both sides so the comparison is platform-neutral.
				gotResolved, err := filepath.EvalSymlinks(got)
				if err != nil {
					t.Fatalf("EvalSymlinks(%q) error = %v", got, err)
				}
				wantResolved, err := filepath.EvalSymlinks(tt.want(cwd))
				if err != nil {
					t.Fatalf("EvalSymlinks(%q) error = %v", tt.want(cwd), err)
				}
				if gotResolved != wantResolved {
					t.Fatalf("FindRoot() = %q, want %q", got, tt.want(cwd))
				}
				return
			}

			if err == nil {
				t.Fatalf("FindRoot() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("FindRoot() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestConfigPath(t *testing.T) {
	root := filepath.Join("/tmp", "repo")
	want := filepath.Join(root, ConfigDir, ConfigFile)

	if got := ConfigPath(root); got != want {
		t.Fatalf("ConfigPath() = %q, want %q", got, want)
	}
}

func writeConfigFile(t *testing.T, root, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Join(root, ConfigDir))
	path := ConfigPath(root)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", path, err)
	}
}

func captureStderr(fn func() error) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()

	original := os.Stderr
	os.Stderr = w
	defer func() {
		os.Stderr = original
	}()

	fnErr := fn()

	if err := w.Close(); err != nil {
		return "", err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(data), fnErr
}
