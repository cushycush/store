package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"unicode/utf8"
)

// TemplateData provides the dot-accessible fields for templates.
type TemplateData struct {
	Hostname string
	OS       string
	Arch     string
	Distro   string
	Shell    string
	Vars     map[string]string
}

var secretPattern = regexp.MustCompile(`\{\{\s*secret\s+"([^"]+)"\s*\}\}`)

// templatePattern matches only the template forms store actually supports:
// the `secret`/`env` function calls and the `.Hostname`/`.OS`/`.Arch`/`.Distro`
// /`.Shell`/`.Vars` data references (including the `index .Vars "..."` form).
// Files that contain unrelated `{{ ... }}` content — GitHub Actions
// expressions, Helm charts, Jinja examples in docs — are left as plain text
// and symlinked verbatim instead of being mis-parsed as Go templates.
var templatePattern = regexp.MustCompile(
	`\{\{[^{}]*?(?:\bsecret\s+"|\benv\s+"|\.(?:Hostname|OS|Arch|Distro|Shell|Vars)\b)`,
)
var varDotPattern = regexp.MustCompile(`\{\{[^{}]*?\.Vars\.([A-Za-z_][A-Za-z0-9_]*)`)
var varIndexPattern = regexp.MustCompile(`\{\{[^{}]*?index\s+\.Vars\s+"([^"]+)"`)

var errTemplatesFound = fmt.Errorf("render: templates found")

// HasSecrets checks if file content contains any {{ secret "..." }} placeholders.
func HasSecrets(content []byte) bool {
	return secretPattern.Match(content)
}

// HasTemplates reports whether content contains any template form that store
// is responsible for rendering. Unrelated `{{ ... }}` content (e.g. literal
// GitHub Actions or Helm syntax in a markdown file) is intentionally not
// treated as a template.
func HasTemplates(content []byte) bool {
	return templatePattern.Match(content)
}

// IsBinary reports whether content looks like binary data unsuitable for
// template rendering. Go's text/template requires valid UTF-8 input and
// binary files that happen to contain the bytes `{{` will crash the parser.
func IsBinary(content []byte) bool {
	if bytes.IndexByte(content, 0) >= 0 {
		return true
	}
	return !utf8.Valid(content)
}

// Render replaces all template expressions in content. Supports:
//   - {{ secret "name" }} — looks up from secrets map
//   - {{ env "VAR" }} — reads from environment
//   - {{ .Hostname }}, {{ .OS }}, {{ .Arch }}, {{ .Distro }}, {{ .Shell }} — platform info
//   - {{ .Vars.key }} — user-defined variables from config
func Render(content []byte, secrets map[string]string, data *TemplateData) ([]byte, error) {
	if data == nil {
		data = &TemplateData{}
	}

	funcMap := template.FuncMap{
		"secret": func(name string) (string, error) {
			val, ok := secrets[name]
			if !ok {
				return "", fmt.Errorf("missing secret: %s", name)
			}
			return val, nil
		},
		"env": os.Getenv,
	}

	tmpl, err := template.New("").
		Option("missingkey=error").
		Funcs(funcMap).
		Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderSecrets is a backward-compatible wrapper that only resolves secret placeholders.
func RenderSecrets(content []byte, secrets map[string]string) ([]byte, error) {
	return Render(content, secrets, nil)
}

// SecretNames extracts all secret names referenced in the content.
func SecretNames(content []byte) []string {
	matches := secretPattern.FindAllSubmatch(content, -1)
	if len(matches) == 0 {
		return []string{}
	}

	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			names = append(names, string(match[1]))
		}
	}

	return names
}

// VarNames extracts all user-defined var names referenced from the config
// `vars:` map. Recognises both `{{ .Vars.key }}` (possibly inside a pipeline)
// and `{{ index .Vars "key" }}` forms.
func VarNames(content []byte) []string {
	names := []string{}
	for _, match := range varDotPattern.FindAllSubmatch(content, -1) {
		if len(match) >= 2 {
			names = append(names, string(match[1]))
		}
	}
	for _, match := range varIndexPattern.FindAllSubmatch(content, -1) {
		if len(match) >= 2 {
			names = append(names, string(match[1]))
		}
	}
	return names
}

// NeedsRendering checks if any file in the given directory tree contains secret placeholders.
func NeedsRendering(dir string) (bool, error) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !IsBinary(content) && HasSecrets(content) {
			return errTemplatesFound
		}

		return nil
	})
	if err == nil {
		return false, nil
	}
	if err == errTemplatesFound {
		return true, nil
	}
	return false, err
}

// NeedsTemplateRendering checks if any file in the directory contains template syntax.
func NeedsTemplateRendering(dir string) (bool, error) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !IsBinary(content) && HasTemplates(content) {
			return errTemplatesFound
		}

		return nil
	})
	if err == nil {
		return false, nil
	}
	if err == errTemplatesFound {
		return true, nil
	}
	return false, err
}

// StagingDir returns the staging directory path for a given repo root.
func StagingDir(repoRoot string) (string, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}

	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}

	hash := sha256.Sum256([]byte(absRoot))
	return filepath.Join(stateHome, "store", hex.EncodeToString(hash[:])), nil
}

// PrepareStaging creates a staging tree for a store directory.
func PrepareStaging(sourceDir, stagingDir string, secrets map[string]string, data *TemplateData) (bool, error) {
	if err := os.RemoveAll(stagingDir); err != nil {
		return false, fmt.Errorf("remove old staging dir: %w", err)
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return false, fmt.Errorf("create staging dir: %w", err)
	}

	renderedAny := false
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("compute relative path for %s: %w", path, err)
		}

		targetPath := stagingDir
		if relPath != "." {
			targetPath = filepath.Join(stagingDir, relPath)
		}

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create parent directory for %s: %w", targetPath, err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		if !IsBinary(content) && HasTemplates(content) {
			rendered, err := Render(content, secrets, data)
			if err != nil {
				return fmt.Errorf("render %s: %w", path, err)
			}
			if err := os.WriteFile(targetPath, rendered, 0o600); err != nil {
				return fmt.Errorf("write rendered file %s: %w", targetPath, err)
			}
			renderedAny = true
			return nil
		}

		if err := os.Symlink(path, targetPath); err != nil {
			return fmt.Errorf("create symlink %s -> %s: %w", targetPath, path, err)
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return renderedAny, nil
}

// CleanStaging removes the staging directory for a repo root.
func CleanStaging(repoRoot string) error {
	stagingDir, err := StagingDir(repoRoot)
	if err != nil {
		return err
	}
	return os.RemoveAll(stagingDir)
}

// ContentHash returns the SHA-256 hex digest of a file's contents.
func ContentHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// FilesMatch returns true if two files have identical content.
func FilesMatch(a, b string) (bool, error) {
	ha, err := ContentHash(a)
	if err != nil {
		return false, err
	}
	hb, err := ContentHash(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

// FormatPlaceholders returns a human-readable list of template expressions in the content.
func FormatPlaceholders(content []byte) []string {
	re := regexp.MustCompile(`\{\{[^}]+\}\}`)
	matches := re.FindAll(content, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		s := strings.TrimSpace(string(m))
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
