package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/ui"
)

func runAdopt(path, name string, dryRun bool, files, patterns []string) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	// Resolve the source path.
	expanded, err := config.ExpandHome(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fi, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is already a symlink (use 'store import' for existing symlinks)", absPath)
	}

	isDir := fi.IsDir()

	// Derive the store name.
	if name == "" {
		name = deriveStoreName(absPath)
	}

	if _, exists := cfg.Stores[name]; exists {
		return fmt.Errorf("store %q already exists (use 'store modify' to update it)", name)
	}

	// Check that the store directory doesn't already exist in the repo.
	storePath := filepath.Join(root, name)
	if _, err := os.Stat(storePath); err == nil {
		return fmt.Errorf("directory %q already exists in repo", name)
	}

	// Build the config entry and determine what will be moved.
	targetPath, err := resolveTargetPath(path)
	if err != nil {
		return err
	}

	var entry config.StoreEntry
	var moveItems []moveItem

	if isDir {
		if len(files) > 0 || len(patterns) > 0 {
			// File-mode adoption: only adopt specific files from the directory.
			entry = config.StoreEntry{Target: targetPath, Files: files, Patterns: patterns}
			items, err := collectFileMoveItems(absPath, files, patterns)
			if err != nil {
				return err
			}
			moveItems = items
		} else {
			// Whole-directory adoption.
			entry = config.StoreEntry{Target: targetPath}
			moveItems = []moveItem{{source: absPath, dest: storePath, isDir: true}}
		}
	} else {
		// Single file adoption: store name directory holds the file,
		// target is the parent directory, files list is just the filename.
		fileName := filepath.Base(absPath)
		parentDir := filepath.Dir(absPath)
		targetPath, err = resolveTargetPath(parentDir)
		if err != nil {
			return err
		}
		entry = config.StoreEntry{Target: targetPath, Files: []string{fileName}}
		moveItems = []moveItem{{
			source: absPath,
			dest:   filepath.Join(storePath, fileName),
		}}
	}

	// Print what we'll do.
	fmt.Println(ui.Bold("Adopting:"))
	if isDir && (len(files) > 0 || len(patterns) > 0) {
		for _, item := range moveItems {
			rel, _ := filepath.Rel(absPath, item.source)
			fmt.Printf("  %s %s %s %s\n", ui.TargetPath(filepath.Join(path, rel)), ui.Arrow(), ui.StoreName(name+"/"+rel), ui.Dim("(file)"))
		}
	} else if isDir {
		fmt.Printf("  %s %s %s %s\n", ui.TargetPath(path), ui.Arrow(), ui.StoreName(name+"/"), ui.Dim("(directory)"))
	} else {
		fmt.Printf("  %s %s %s %s\n", ui.TargetPath(path), ui.Arrow(), ui.StoreName(name+"/"+filepath.Base(absPath)), ui.Dim("(file)"))
	}

	if dryRun {
		fmt.Printf("\n%s\n", ui.Dim("Dry run: no changes made"))
		return nil
	}

	if !forceBackups && !promptYesNo("Proceed?") {
		return fmt.Errorf("aborted")
	}

	// Execute the adoption.
	if err := executeMoves(storePath, moveItems); err != nil {
		return err
	}

	// Write config.
	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create symlinks.
	if isDir && len(files) == 0 && len(patterns) == 0 {
		// Whole-directory: symlink the directory itself.
		if err := linker.Link(storePath, absPath); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	} else {
		// File mode: symlink each file.
		for _, item := range moveItems {
			if err := linker.Link(item.dest, item.source); err != nil {
				return fmt.Errorf("failed to create symlink for %s: %w", item.source, err)
			}
		}
	}

	printStoredTarget(name, entry.Target, entry.HasFileMode())
	return nil
}

type moveItem struct {
	source string // original location (will become the symlink)
	dest   string // destination in repo
	isDir  bool
}

// deriveStoreName extracts a store name from a path.
// It strips leading dots from the basename so ~/.zshrc becomes "zshrc".
func deriveStoreName(absPath string) string {
	base := filepath.Base(absPath)
	return strings.TrimLeft(base, ".")
}

// collectFileMoveItems resolves explicit files and patterns against a directory
// and returns the list of individual files to move.
func collectFileMoveItems(dir string, files, patterns []string) ([]moveItem, error) {
	seen := make(map[string]bool)
	var items []moveItem

	for _, f := range files {
		abs := filepath.Join(dir, f)
		if _, err := os.Lstat(abs); err != nil {
			return nil, fmt.Errorf("file %q not found in %s", f, dir)
		}
		if !seen[f] {
			seen[f] = true
			items = append(items, moveItem{source: abs, dest: ""}) // dest set during execute
		}
	}

	for _, pattern := range patterns {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, path)
			matched, matchErr := filepath.Match(pattern, rel)
			if matchErr != nil {
				return matchErr
			}
			// Also try matching just the filename for simple patterns.
			if !matched {
				matched, _ = filepath.Match(pattern, filepath.Base(rel))
			}
			if matched && !seen[rel] {
				seen[rel] = true
				items = append(items, moveItem{source: path, dest: ""})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", pattern, err)
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no files matched in %s", dir)
	}

	return items, nil
}

// executeMoves moves source files/directories into the repo store directory.
func executeMoves(storePath string, items []moveItem) error {
	for i, item := range items {
		if item.isDir {
			// Whole directory: rename directly to store path.
			if err := os.Rename(item.source, storePath); err != nil {
				return fmt.Errorf("failed to move %s to %s: %w", item.source, storePath, err)
			}
			continue
		}

		// Individual file: ensure dest is set and parent dirs exist.
		dest := item.dest
		if dest == "" {
			rel, _ := filepath.Rel(filepath.Dir(item.source), item.source)
			dest = filepath.Join(storePath, rel)
			items[i].dest = dest
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", dest, err)
		}
		if err := os.Rename(item.source, dest); err != nil {
			return fmt.Errorf("failed to move %s to %s: %w", item.source, dest, err)
		}
	}

	return nil
}
