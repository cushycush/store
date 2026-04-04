package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cush/store/internal/config"
	"github.com/cush/store/internal/linker"
	"github.com/cush/store/internal/matcher"
)

// ConflictInfo describes a file or directory that conflicts with a store symlink.
type ConflictInfo struct {
	Source string // Path in the store directory (where the file will be moved to).
	Target string // Path on the filesystem (the conflicting file/dir).
	IsDir  bool   // Whether the conflict is a directory.
}

// CollectTargetConflicts checks all source/target pairs for a single target entry
// and returns any that have StatusConflict.
func CollectTargetConflicts(root string, name string, te config.TargetEntry) ([]ConflictInfo, error) {
	source := filepath.Join(root, name)
	target, err := config.ExpandHome(te.Target)
	if err != nil {
		return nil, fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	if !te.HasFileMode() {
		// Whole-directory mode: check the single directory symlink.
		status, err := linker.Check(source, target)
		if err != nil {
			return nil, fmt.Errorf("store %q target %q: %w", name, te.Target, err)
		}
		if status == linker.StatusConflict {
			fi, err := os.Lstat(target)
			if err != nil {
				return nil, fmt.Errorf("failed to stat %s: %w", target, err)
			}
			return []ConflictInfo{{
				Source: source,
				Target: target,
				IsDir:  fi.IsDir(),
			}}, nil
		}
		return nil, nil
	}

	// File mode: check each matched file.
	files, err := matcher.Match(source, te.Files, te.Patterns)
	if err != nil {
		return nil, fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	var conflicts []ConflictInfo
	for _, rel := range files {
		src := filepath.Join(source, rel)
		tgt := filepath.Join(target, rel)
		status, err := linker.Check(src, tgt)
		if err != nil {
			return nil, fmt.Errorf("store %q target %q file %q: %w", name, te.Target, rel, err)
		}
		if status == linker.StatusConflict {
			fi, err := os.Lstat(tgt)
			if err != nil {
				return nil, fmt.Errorf("failed to stat %s: %w", tgt, err)
			}
			conflicts = append(conflicts, ConflictInfo{
				Source: src,
				Target: tgt,
				IsDir:  fi.IsDir(),
			})
		}
	}
	return conflicts, nil
}

// CollectConflicts checks all targets in a store entry for conflicts.
func CollectConflicts(root string, name string, entry config.StoreEntry) ([]ConflictInfo, error) {
	var all []ConflictInfo
	for _, te := range entry.ResolvedTargets() {
		conflicts, err := CollectTargetConflicts(root, name, te)
		if err != nil {
			return nil, err
		}
		all = append(all, conflicts...)
	}
	return all, nil
}

// ResolveConflicts moves conflicting files/directories from the target location
// into the store source location. If a file already exists at the source, it is
// backed up with a .bak suffix before the move.
func ResolveConflicts(conflicts []ConflictInfo) error {
	for _, c := range conflicts {
		if c.IsDir {
			if err := resolveDirectoryConflict(c.Source, c.Target); err != nil {
				return fmt.Errorf("failed to resolve conflict for %s: %w", c.Target, err)
			}
		} else {
			if err := resolveFileConflict(c.Source, c.Target); err != nil {
				return fmt.Errorf("failed to resolve conflict for %s: %w", c.Target, err)
			}
		}
	}
	return nil
}

// resolveFileConflict moves a single conflicting file from target into source.
// If source already exists, it is backed up first.
func resolveFileConflict(source, target string) error {
	if err := backupIfExists(source); err != nil {
		return err
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(source), err)
	}

	if err := os.Rename(target, source); err != nil {
		return fmt.Errorf("failed to move %s to %s: %w", target, source, err)
	}
	return nil
}

// resolveDirectoryConflict merges the contents of a conflicting target directory
// into the store source directory. Files that already exist in source are backed
// up before being overwritten.
func resolveDirectoryConflict(source, target string) error {
	// Walk the target directory and move each file into the source.
	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}

		srcPath := filepath.Join(source, rel)

		if d.IsDir() {
			// Create the corresponding directory in source.
			return os.MkdirAll(srcPath, 0o755)
		}

		// It's a file — backup existing source file if needed, then move.
		if err := backupIfExists(srcPath); err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(srcPath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(srcPath), err)
		}

		return os.Rename(path, srcPath)
	})
	if err != nil {
		return fmt.Errorf("failed to merge directory %s into %s: %w", target, source, err)
	}

	// Remove the now-empty target directory tree.
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("failed to remove %s after merge: %w", target, err)
	}
	return nil
}

// backupIfExists renames an existing file/directory by appending a .bak suffix.
// If the .bak path also exists, it tries .bak.1, .bak.2, etc.
func backupIfExists(path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil // Nothing to back up.
	} else if err != nil {
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}

	bakPath := path + ".bak"
	if _, err := os.Lstat(bakPath); os.IsNotExist(err) {
		return os.Rename(path, bakPath)
	}

	for i := 1; ; i++ {
		numbered := fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Lstat(numbered); os.IsNotExist(err) {
			return os.Rename(path, numbered)
		}
	}
}
