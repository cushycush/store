package render

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestHasSecrets(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "standard spacing", content: `{{ secret "foo" }}`, want: true},
		{name: "tight spacing", content: `{{secret "bar"}}`, want: true},
		{name: "extra spacing", content: `{{  secret  "baz"  }}`, want: true},
		{name: "multiple secrets", content: `before {{ secret "one" }} middle {{secret "two"}} after`, want: true},
		{name: "plain text", content: `hello world`, want: false},
		{name: "different helper", content: `{{ notasecret "x" }}`, want: false},
		{name: "empty content", content: ``, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasSecrets([]byte(tt.content)); got != tt.want {
				t.Fatalf("HasSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasTemplates(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		// Recognised store template forms.
		{name: "secret call", content: `{{ secret "token" }}`, want: true},
		{name: "secret call tight", content: `{{secret "token"}}`, want: true},
		{name: "env call", content: `{{ env "HOME" }}`, want: true},
		{name: "hostname", content: `host={{ .Hostname }}`, want: true},
		{name: "os", content: `{{ .OS }}`, want: true},
		{name: "arch", content: `{{ .Arch }}`, want: true},
		{name: "distro", content: `{{ .Distro }}`, want: true},
		{name: "shell", content: `{{ .Shell }}`, want: true},
		{name: "vars dot", content: `editor={{ .Vars.editor }}`, want: true},
		{name: "vars dot pipeline", content: `{{ .Vars.name | upper }}`, want: true},
		{name: "vars index", content: `{{ index .Vars "my-key" }}`, want: true},
		{name: "trim markers", content: `{{- .Hostname -}}`, want: true},
		{name: "control flow with .OS", content: `{{if eq .OS "linux"}}foo{{end}}`, want: true},
		{name: "with .Vars block", content: `{{with .Vars}}{{.editor}}{{end}}`, want: true},

		// Literal `{{ ... }}` content from other tools — must NOT be treated
		// as a store template. These are the regressions reported in #58.
		{name: "github actions secrets", content: `github_token: ${{ secrets.GITHUB_TOKEN }}`, want: false},
		{name: "github actions inputs", content: `name: ${{ inputs.name }}`, want: false},
		{name: "helm values", content: `replicas: {{ .Values.replicaCount }}`, want: false},
		{name: "helm release", content: `name: {{ .Release.Name }}`, want: false},
		{name: "jinja variable", content: `Hello {{ name }}`, want: false},
		{name: "handlebars", content: `{{firstName}} {{lastName}}`, want: false},
		{name: "literal docs", content: "Use `{{ ... }}` for templates.", want: false},
		{name: "unbalanced opener", content: `prefix {{ no closer here`, want: false},
		{name: "vars-like prefix", content: `{{ .Variables.foo }}`, want: false},
		{name: "os-like prefix", content: `{{ .OSDetails.kernel }}`, want: false},

		// Plain content.
		{name: "plain text", content: `hello world`, want: false},
		{name: "empty", content: ``, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasTemplates([]byte(tt.content)); got != tt.want {
				t.Fatalf("HasTemplates(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{name: "plain text", content: []byte("hello world"), want: false},
		{name: "utf-8 text", content: []byte("héllo 世界"), want: false},
		{name: "template syntax", content: []byte(`{{ secret "x" }}`), want: false},
		{name: "contains NUL byte", content: []byte("abc\x00def"), want: true},
		{name: "PNG-like header", content: []byte("\x89PNG\r\n\x1a\n{{IHDR"), want: true},
		{name: "invalid utf-8", content: []byte{0xff, 0xfe, 0xfd}, want: true},
		{name: "empty content", content: []byte{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBinary(tt.content); got != tt.want {
				t.Fatalf("IsBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		content string
		secrets map[string]string
		want    string
		wantErr string
	}{
		{
			name:    "single substitution",
			content: `token={{ secret "api_key" }}`,
			secrets: map[string]string{"api_key": "abc123"},
			want:    `token=abc123`,
		},
		{
			name:    "multiple substitutions",
			content: `{{secret "first"}}/{{ secret "second" }}`,
			secrets: map[string]string{"first": "one", "second": "two"},
			want:    `one/two`,
		},
		{
			name:    "missing secret lists name",
			content: `{{ secret "missing" }}`,
			secrets: map[string]string{},
			wantErr: `missing secret: missing`,
		},
		{
			name:    "all missing secrets are listed",
			content: `{{ secret "beta" }} {{ secret "alpha" }} {{ secret "beta" }}`,
			secrets: map[string]string{},
			wantErr: `missing secret: beta`,
		},
		{
			name:    "no placeholders with empty secrets map succeeds",
			content: `plain text`,
			secrets: map[string]string{},
			want:    `plain text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render([]byte(tt.content), tt.secrets, nil)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Render() error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Render() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Render() unexpected error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Render() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestSecretNames(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "extracts all names", content: `{{ secret "one" }} and {{secret "two"}}`, want: []string{"one", "two"}},
		{name: "returns empty slice for no matches", content: `plain text`, want: []string{}},
		{name: "returns duplicate occurrences", content: `{{ secret "dup" }} {{ secret "dup" }}`, want: []string{"dup", "dup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SecretNames([]byte(tt.content))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SecretNames() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestVarNames(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "single dot form", content: `hello {{ .Vars.name }}`, want: []string{"name"}},
		{name: "tight spacing", content: `{{.Vars.editor}}`, want: []string{"editor"}},
		{name: "pipeline suffix", content: `{{ .Vars.name | lower }}`, want: []string{"name"}},
		{name: "index form with dashed key", content: `{{ index .Vars "my-key" }}`, want: []string{"my-key"}},
		{name: "multiple mixed forms", content: `{{ .Vars.a }} and {{ index .Vars "b" }} and {{.Vars.c | upper}}`, want: []string{"a", "c", "b"}},
		{name: "no matches", content: `plain {{ .Hostname }} {{ secret "x" }}`, want: []string{}},
		{name: "dotted path keeps first segment", content: `{{ .Vars.foo.bar }}`, want: []string{"foo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VarNames([]byte(tt.content))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("VarNames() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNeedsRendering(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name: "true when file contains secret",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "config.txt"), `{{ secret "token" }}`)
			},
			want: true,
		},
		{
			name: "false when none do",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "config.txt"), `plain text`)
				writeTestFile(t, filepath.Join(dir, "notes.md"), `still plain text`)
			},
			want: false,
		},
		{
			name: "handles nested directories",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "nested", "deep", "secret.txt"), `{{secret "nested"}}`)
			},
			want: true,
		},
		{
			name: "ignores binary files that contain template bytes",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "image.png"), "\x89PNG\r\n\x1a\n{{ secret \"x\" }}")
			},
			want: false,
		},
		{
			name: "skips directories themselves",
			setup: func(t *testing.T, dir string) {
				if runtime.GOOS == "windows" {
					// NTFS disallows `"` in filenames, so fall back to a
					// template-free directory name that still exercises the
					// "non-template directory is skipped" path.
					if err := os.MkdirAll(filepath.Join(dir, "plain-dir"), 0o755); err != nil {
						t.Fatalf("MkdirAll() failed: %v", err)
					}
				} else {
					if err := os.MkdirAll(filepath.Join(dir, "{{ secret \"dir\" }}"), 0o755); err != nil {
						t.Fatalf("MkdirAll() failed: %v", err)
					}
				}
				writeTestFile(t, filepath.Join(dir, "plain.txt"), `no secrets here`)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			got, err := NeedsRendering(dir)
			if err != nil {
				t.Fatalf("NeedsRendering() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NeedsRendering() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStagingDir(t *testing.T) {
	tests := []struct {
		name        string
		setEnv      func(t *testing.T)
		makeRoots   func(t *testing.T) (string, string)
		wantPrefix  func() string
		checkUnique bool
	}{
		{
			name: "uses XDG_STATE_HOME when set",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
			},
			makeRoots: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			wantPrefix: func() string {
				return os.Getenv("XDG_STATE_HOME")
			},
		},
		{
			name: "uses local state fallback",
			setEnv: func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("XDG_STATE_HOME", "")
				t.Setenv("HOME", home)
				// os.UserHomeDir() reads USERPROFILE on Windows; mirror HOME
				// into it so the fallback computation agrees with the test.
				t.Setenv("USERPROFILE", home)
			},
			makeRoots: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			wantPrefix: func() string {
				home := os.Getenv("HOME")
				if runtime.GOOS == "windows" {
					home = os.Getenv("USERPROFILE")
				}
				return filepath.Join(home, ".local", "state")
			},
		},
		{
			name: "different roots produce different paths",
			setEnv: func(t *testing.T) {
				t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
			},
			makeRoots: func(t *testing.T) (string, string) {
				base := t.TempDir()
				return filepath.Join(base, "repo-one"), filepath.Join(base, "repo-two")
			},
			checkUnique: true,
			wantPrefix: func() string {
				return os.Getenv("XDG_STATE_HOME")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setEnv(t)
			repoRoot, otherRoot := tt.makeRoots(t)

			got, err := StagingDir(repoRoot)
			if err != nil {
				t.Fatalf("StagingDir() error = %v", err)
			}

			absRoot, err := filepath.Abs(repoRoot)
			if err != nil {
				t.Fatalf("filepath.Abs() error = %v", err)
			}
			hash := sha256.Sum256([]byte(absRoot))
			want := filepath.Join(tt.wantPrefix(), "store", hex.EncodeToString(hash[:]))
			if got != want {
				t.Fatalf("StagingDir() = %q, want %q", got, want)
			}

			if tt.checkUnique {
				other, err := StagingDir(otherRoot)
				if err != nil {
					t.Fatalf("StagingDir(otherRoot) error = %v", err)
				}
				if other == got {
					t.Fatalf("StagingDir() for %q and %q should differ", repoRoot, otherRoot)
				}
			}
		})
	}
}

func TestPrepareStaging(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, sourceDir, stagingDir string)
		secrets   map[string]string
		want      bool
		wantErr   string
		assertion func(t *testing.T, sourceDir, stagingDir string)
	}{
		{
			name: "renders templates and symlinks non template files",
			setup: func(t *testing.T, sourceDir, stagingDir string) {
				writeTestFile(t, filepath.Join(sourceDir, "secret.txt"), `token={{ secret "token" }}`)
				writeTestFile(t, filepath.Join(sourceDir, "plain.txt"), `plain file`)
				writeTestFile(t, filepath.Join(stagingDir, "stale.txt"), `stale`)
			},
			secrets: map[string]string{"token": "abc123"},
			want:    true,
			assertion: func(t *testing.T, sourceDir, stagingDir string) {
				renderedPath := filepath.Join(stagingDir, "secret.txt")
				assertFileContent(t, renderedPath, `token=abc123`)

				info, err := os.Stat(renderedPath)
				if err != nil {
					t.Fatalf("Stat(%q) failed: %v", renderedPath, err)
				}
				// Windows doesn't honor POSIX permission bits on os.WriteFile,
				// so only assert the restrictive 0o600 mode on POSIX.
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
					t.Fatalf("rendered file mode = %o, want 600", info.Mode().Perm())
				}

				assertSymlinkTarget(t, filepath.Join(stagingDir, "plain.txt"), filepath.Join(sourceDir, "plain.txt"))
				assertMissing(t, filepath.Join(stagingDir, "stale.txt"))
			},
		},
		{
			name: "returns false when none need rendering",
			setup: func(t *testing.T, sourceDir, stagingDir string) {
				writeTestFile(t, filepath.Join(sourceDir, "plain.txt"), `plain`)
			},
			secrets: map[string]string{},
			want:    false,
			assertion: func(t *testing.T, sourceDir, stagingDir string) {
				assertSymlinkTarget(t, filepath.Join(stagingDir, "plain.txt"), filepath.Join(sourceDir, "plain.txt"))
			},
		},
		{
			name: "handles nested directory structures",
			setup: func(t *testing.T, sourceDir, stagingDir string) {
				writeTestFile(t, filepath.Join(sourceDir, "nested", "deep", "config.txt"), `{{ secret "name" }}`)
				writeTestFile(t, filepath.Join(sourceDir, "nested", "deep", "plain.txt"), `plain`)
			},
			secrets: map[string]string{"name": "value"},
			want:    true,
			assertion: func(t *testing.T, sourceDir, stagingDir string) {
				assertFileContent(t, filepath.Join(stagingDir, "nested", "deep", "config.txt"), `value`)
				assertSymlinkTarget(t, filepath.Join(stagingDir, "nested", "deep", "plain.txt"), filepath.Join(sourceDir, "nested", "deep", "plain.txt"))
			},
		},
		{
			name: "missing secrets cause error",
			setup: func(t *testing.T, sourceDir, stagingDir string) {
				writeTestFile(t, filepath.Join(sourceDir, "secret.txt"), `{{ secret "missing" }}`)
			},
			secrets: map[string]string{},
			wantErr: `missing secret: missing`,
		},
		{
			name: "literal github actions syntax is symlinked, not rendered",
			setup: func(t *testing.T, sourceDir, stagingDir string) {
				writeTestFile(t, filepath.Join(sourceDir, "workflow.md"),
					"Example:\n\n    github_token: ${{ secrets.GITHUB_TOKEN }}\n")
			},
			secrets: map[string]string{},
			want:    false,
			assertion: func(t *testing.T, sourceDir, stagingDir string) {
				assertSymlinkTarget(t,
					filepath.Join(stagingDir, "workflow.md"),
					filepath.Join(sourceDir, "workflow.md"))
			},
		},
		{
			name: "binary files with template-looking bytes are symlinked, not rendered",
			setup: func(t *testing.T, sourceDir, stagingDir string) {
				writeTestFile(t, filepath.Join(sourceDir, "image.png"), "\x89PNG\r\n\x1a\n{{IHDR\x00payload")
			},
			secrets: map[string]string{},
			want:    false,
			assertion: func(t *testing.T, sourceDir, stagingDir string) {
				assertSymlinkTarget(t, filepath.Join(stagingDir, "image.png"), filepath.Join(sourceDir, "image.png"))
			},
		},
		{
			name:    "empty directory returns false",
			setup:   func(t *testing.T, sourceDir, stagingDir string) {},
			secrets: map[string]string{},
			want:    false,
			assertion: func(t *testing.T, sourceDir, stagingDir string) {
				entries, err := os.ReadDir(stagingDir)
				if err != nil {
					t.Fatalf("ReadDir(%q) failed: %v", stagingDir, err)
				}
				if len(entries) != 0 {
					t.Fatalf("staging dir entries = %d, want 0", len(entries))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			stagingDir := filepath.Join(t.TempDir(), "staging")
			tt.setup(t, sourceDir, stagingDir)

			got, err := PrepareStaging(sourceDir, stagingDir, tt.secrets, nil)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("PrepareStaging() error = nil, want substring %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("PrepareStaging() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("PrepareStaging() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("PrepareStaging() = %v, want %v", got, tt.want)
			}
			if tt.assertion != nil {
				tt.assertion(t, sourceDir, stagingDir)
			}
		})
	}
}

func TestCleanStaging(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repoRoot string) string
	}{
		{
			name: "removes staging directory",
			setup: func(t *testing.T, repoRoot string) string {
				stagingDir, err := StagingDir(repoRoot)
				if err != nil {
					t.Fatalf("StagingDir() error = %v", err)
				}
				writeTestFile(t, filepath.Join(stagingDir, "rendered.txt"), `secret`)
				return stagingDir
			},
		},
		{
			name: "no error when already missing",
			setup: func(t *testing.T, repoRoot string) string {
				stagingDir, err := StagingDir(repoRoot)
				if err != nil {
					t.Fatalf("StagingDir() error = %v", err)
				}
				return stagingDir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
			repoRoot := t.TempDir()
			stagingDir := tt.setup(t, repoRoot)

			if err := CleanStaging(repoRoot); err != nil {
				t.Fatalf("CleanStaging() error = %v", err)
			}

			assertMissing(t, stagingDir)
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("content of %q = %q, want %q", path, string(data), want)
	}
}

func assertSymlinkTarget(t *testing.T, linkPath, wantTarget string) {
	t.Helper()

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat(%q) failed: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%q is not a symlink", linkPath)
	}

	gotTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink(%q) failed: %v", linkPath, err)
	}
	if gotTarget != wantTarget {
		t.Fatalf("symlink %q points to %q, want %q", linkPath, gotTarget, wantTarget)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to be missing, got err=%v", path, err)
	}
}
