package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cush/store/internal/config"
	"github.com/cush/store/internal/linker"
	"github.com/cush/store/internal/matcher"
)

// StoreTarget creates symlinks for a single target entry within a store.
func StoreTarget(root string, name string, te config.TargetEntry) error {
	source := filepath.Join(root, name)
	target, err := config.ExpandHome(te.Target)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	if !te.HasFileMode() {
		if err := linker.Link(source, target); err != nil {
			return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
		}
		return nil
	}

	// File mode: resolve matches and link each file.
	files, err := matcher.Match(source, te.Files, te.Patterns)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	var errors []error
	for _, rel := range files {
		src := filepath.Join(source, rel)
		tgt := filepath.Join(target, rel)
		if err := linker.Link(src, tgt); err != nil {
			errors = append(errors, fmt.Errorf("  %s: %w", rel, err))
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("store %q target %q: %d file(s) failed", name, te.Target, len(errors))
	}

	return nil
}

// Store creates symlinks for a single store entry (all targets).
func Store(root string, name string, entry config.StoreEntry) error {
	targets := entry.ResolvedTargets()
	if len(targets) == 0 {
		return nil // No targets configured yet; skip silently.
	}

	var errors []error
	for _, te := range targets {
		if err := StoreTarget(root, name, te); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("store %q: %d target(s) failed", name, len(errors))
	}

	return nil
}

// StoreAll creates symlinks for all stores in the config.
func StoreAll(root string, cfg *config.Config) error {
	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	var errors []error
	for name, entry := range cfg.Stores {
		if err := Store(root, name, entry); err != nil {
			errors = append(errors, err)
		} else {
			for _, te := range entry.ResolvedTargets() {
				if te.HasFileMode() {
					fmt.Printf("  %s -> %s (files)\n", name, te.Target)
				} else {
					fmt.Printf("  %s -> %s\n", name, te.Target)
				}
			}
		}
	}

	if len(errors) > 0 {
		fmt.Println()
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("%d store(s) failed", len(errors))
	}

	return nil
}

// StoreRemoveTarget removes symlinks for a single target entry within a store.
func StoreRemoveTarget(root string, name string, te config.TargetEntry) error {
	source := filepath.Join(root, name)
	target, err := config.ExpandHome(te.Target)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	if !te.HasFileMode() {
		if err := linker.Unlink(source, target); err != nil {
			return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
		}
		return nil
	}

	// File mode: resolve matches and unlink each file.
	files, err := matcher.Match(source, te.Files, te.Patterns)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	var errors []error
	for _, rel := range files {
		src := filepath.Join(source, rel)
		tgt := filepath.Join(target, rel)
		if err := linker.Unlink(src, tgt); err != nil {
			errors = append(errors, fmt.Errorf("  %s: %w", rel, err))
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("store %q target %q: %d file(s) failed to unlink", name, te.Target, len(errors))
	}

	cleanupEmptyDirs(target, files)
	return nil
}

// StoreRemove removes symlinks for a single store (all targets).
func StoreRemove(root string, name string, entry config.StoreEntry) error {
	targets := entry.ResolvedTargets()
	if len(targets) == 0 {
		return nil
	}

	var errors []error
	for _, te := range targets {
		if err := StoreRemoveTarget(root, name, te); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("store %q: %d target(s) failed to unlink", name, len(errors))
	}

	return nil
}

// cleanupEmptyDirs removes empty directories under target that were created as
// parents of file-mode symlinks. It processes paths deepest-first and only
// removes directories that are empty, so it is always safe to call.
func cleanupEmptyDirs(target string, relPaths []string) {
	// Collect unique parent directories, deepest first.
	dirs := make(map[string]struct{})
	for _, rel := range relPaths {
		dir := filepath.Dir(rel)
		for dir != "." && dir != "" {
			dirs[dir] = struct{}{}
			dir = filepath.Dir(dir)
		}
	}

	// Sort descending by depth so we remove children before parents.
	sorted := make([]string, 0, len(dirs))
	for d := range dirs {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool {
		// More path separators = deeper; break ties lexicographically reversed.
		di := len(filepath.SplitList(sorted[i]))
		dj := len(filepath.SplitList(sorted[j]))
		if di != dj {
			return di > dj
		}
		return sorted[i] > sorted[j]
	})

	for _, dir := range sorted {
		full := filepath.Join(target, dir)
		// os.Remove only removes empty directories, so this is safe.
		os.Remove(full)
	}

	// Finally, try to remove the target directory itself if it is now empty.
	// os.Remove fails on non-empty directories, so this is safe — it will
	// not remove directories like ~ that still contain other files.
	os.Remove(target)
}

// StoreRemoveAll removes symlinks for all stores in the config.
func StoreRemoveAll(root string, cfg *config.Config) error {
	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	var errors []error
	for name, entry := range cfg.Stores {
		if err := StoreRemove(root, name, entry); err != nil {
			errors = append(errors, err)
		} else {
			for _, te := range entry.ResolvedTargets() {
				fmt.Printf("  removed %s (%s)\n", name, te.Target)
			}
		}
	}

	if len(errors) > 0 {
		fmt.Println()
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("%d store(s) failed", len(errors))
	}

	return nil
}

// StatusInfo holds the status of a single store or file within a store.
type StatusInfo struct {
	Name   string
	File   string // Non-empty when reporting per-file status.
	Target string
	Status linker.Status
	Error  error
}

// GetStatus checks the symlink status of a single store (all targets).
// For file-mode targets, it returns one StatusInfo per matched file.
func GetStatus(root string, name string, entry config.StoreEntry) []StatusInfo {
	targets := entry.ResolvedTargets()
	if len(targets) == 0 {
		return []StatusInfo{{
			Name:  name,
			Error: fmt.Errorf("no target configured"),
		}}
	}

	source := filepath.Join(root, name)
	var results []StatusInfo

	for _, te := range targets {
		target, err := config.ExpandHome(te.Target)
		if err != nil {
			results = append(results, StatusInfo{Name: name, Target: te.Target, Error: err})
			continue
		}

		if !te.HasFileMode() {
			info := StatusInfo{
				Name:   name,
				Target: te.Target,
			}
			status, err := linker.Check(source, target)
			if err != nil {
				info.Error = err
			} else {
				info.Status = status
			}
			results = append(results, info)
			continue
		}

		// File mode: check each matched file.
		files, err := matcher.Match(source, te.Files, te.Patterns)
		if err != nil {
			results = append(results, StatusInfo{Name: name, Target: te.Target, Error: err})
			continue
		}

		for _, rel := range files {
			src := filepath.Join(source, rel)
			tgt := filepath.Join(target, rel)
			info := StatusInfo{
				Name:   name,
				File:   rel,
				Target: filepath.Join(te.Target, rel),
			}
			status, err := linker.Check(src, tgt)
			if err != nil {
				info.Error = err
			} else {
				info.Status = status
			}
			results = append(results, info)
		}
	}

	return results
}

// GetStatusAll checks the symlink status of all stores.
func GetStatusAll(root string, cfg *config.Config) []StatusInfo {
	var results []StatusInfo
	for name, entry := range cfg.Stores {
		results = append(results, GetStatus(root, name, entry)...)
	}
	return results
}
