package matcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Match resolves explicit file paths and glob patterns against a store directory,
// returning a deduplicated, sorted list of relative paths.
//
// Performance strategy:
//   - Explicit files are validated with os.Lstat (no directory walk).
//   - Simple patterns (no "**") use doublestar.Glob (single-level scan).
//   - Only patterns containing "**" trigger a full recursive walk via GlobWalk.
func Match(storeDir string, files []string, patterns []string) ([]string, error) {
	seen := make(map[string]struct{})

	// Resolve explicit files first — no walking required.
	for _, f := range files {
		if f == "" {
			continue
		}

		// Prevent path traversal outside the store directory.
		clean := filepath.Clean(f)
		if clean == "." || strings.HasPrefix(clean, "..") {
			return nil, fmt.Errorf("file path %q escapes store directory", f)
		}

		full := filepath.Join(storeDir, clean)
		if _, err := os.Lstat(full); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file %q not found in store directory", f)
			}
			return nil, fmt.Errorf("failed to stat %q: %w", f, err)
		}
		seen[clean] = struct{}{}
	}

	// Resolve patterns.
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		clean := filepath.Clean(pattern)
		if clean == "." || strings.HasPrefix(clean, "..") {
			return nil, fmt.Errorf("pattern %q escapes store directory", pattern)
		}

		if strings.Contains(pattern, "**") {
			// Recursive pattern — use GlobWalk for efficient single-pass matching.
			fsys := os.DirFS(storeDir)
			err := doublestar.GlobWalk(fsys, pattern, func(path string, d os.DirEntry) error {
				// Skip directories themselves; we only symlink files.
				if d.IsDir() {
					return nil
				}
				seen[path] = struct{}{}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("pattern %q: %w", pattern, err)
			}
		} else {
			// Non-recursive pattern — Glob without a full walk.
			fsys := os.DirFS(storeDir)
			matches, err := doublestar.Glob(fsys, pattern)
			if err != nil {
				return nil, fmt.Errorf("pattern %q: %w", pattern, err)
			}
			for _, m := range matches {
				// Only include files, not directories.
				full := filepath.Join(storeDir, m)
				fi, err := os.Lstat(full)
				if err != nil {
					continue
				}
				if !fi.IsDir() {
					seen[m] = struct{}{}
				}
			}
		}
	}

	// Collect and sort results for deterministic output.
	result := make([]string, 0, len(seen))
	for path := range seen {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}
