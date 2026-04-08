package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var secretPattern = regexp.MustCompile(`\{\{\s*secret\s+"([^"]+)"\s*\}\}`)

var errSecretsFound = fmt.Errorf("render: secrets found")

// HasSecrets checks if file content contains any {{ secret "..." }} placeholders.
func HasSecrets(content []byte) bool {
	return secretPattern.Match(content)
}

// Render replaces all {{ secret "name" }} placeholders in content with values from the secrets map.
// Returns an error if a referenced secret is not found in the map.
func Render(content []byte, secrets map[string]string) ([]byte, error) {
	missing := make(map[string]struct{})

	rendered := secretPattern.ReplaceAllFunc(content, func(match []byte) []byte {
		submatches := secretPattern.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		name := string(submatches[1])
		value, ok := secrets[name]
		if !ok {
			missing[name] = struct{}{}
			return match
		}

		return []byte(value)
	})

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("missing secrets: %s", strings.Join(names, ", "))
	}

	return rendered, nil
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
		if HasSecrets(content) {
			return errSecretsFound
		}

		return nil
	})
	if err == nil {
		return false, nil
	}
	if err == errSecretsFound {
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
func PrepareStaging(sourceDir, stagingDir string, secrets map[string]string) (bool, error) {
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

		if HasSecrets(content) {
			rendered, err := Render(content, secrets)
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
