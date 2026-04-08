package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/hooks"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/matcher"
	"github.com/cushycush/store/internal/render"
)

// needsAutoPromotion checks if the source directory contains files/dirs
// that match global ignore patterns, requiring promotion from whole-directory
// to file-mode symlinking.
func needsAutoPromotion(source string) bool {
	for _, name := range []string{".store", ".git", ".gitignore", ".DS_Store"} {
		if _, err := os.Lstat(filepath.Join(source, name)); err == nil {
			return true
		}
	}
	return false
}

func shouldUseFileMode(source string, te config.TargetEntry) bool {
	return te.HasFileMode() || len(te.Ignore) > 0 || needsAutoPromotion(source)
}

func resolveTargetMatches(source string, te config.TargetEntry) ([]string, error) {
	files := te.Files
	patterns := te.Patterns
	if !te.HasFileMode() {
		patterns = []string{"**/*"}
	}
	return matcher.Match(source, files, patterns, te.Ignore)
}

func linkTarget(source, name, expandedTarget string, te config.TargetEntry) error {
	if !shouldUseFileMode(source, te) {
		if err := linker.Link(source, expandedTarget); err != nil {
			return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
		}
		return nil
	}

	files, err := resolveTargetMatches(source, te)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	var errors []error
	for _, rel := range files {
		src := filepath.Join(source, rel)
		tgt := filepath.Join(expandedTarget, rel)
		if err := linker.Link(src, tgt); err != nil {
			errors = append(errors, fmt.Errorf("  %s: %w", rel, err))
		}
	}

	if len(errors) > 0 {
		printErrors(errors)
		return fmt.Errorf("store %q target %q: %d file(s) failed", name, te.Target, len(errors))
	}

	return nil
}

func printErrors(errors []error) {
	for _, err := range errors {
		fmt.Printf("  error: %s\n", err)
	}
}

func runStoreTargets(root, name string, entry config.StoreEntry, action string, runTarget func(config.TargetEntry) error, failureFmt string) error {
	targets := entry.ResolvedTargets()
	if len(targets) == 0 {
		return nil
	}

	targetStr := targets[0].Target
	if err := hooks.RunEntry(root, name, targetStr, action, "pre", entry.Hooks); err != nil {
		return err
	}

	var errors []error
	for _, te := range targets {
		if err := runTarget(te); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		printErrors(errors)
		return fmt.Errorf(failureFmt, name, len(errors))
	}

	if err := hooks.RunEntry(root, name, targetStr, action, "post", entry.Hooks); err != nil {
		fmt.Printf("  warning: %s\n", err)
	}

	return nil
}

// StoreTarget creates symlinks for a single target entry within a store.
func StoreTarget(root string, name string, te config.TargetEntry) error {
	source := filepath.Join(root, name)
	target, err := config.ExpandHome(te.Target)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	return linkTarget(source, name, target, te)
}

// resolveSource returns the effective source directory for a store.
// If secrets are provided and the store contains template files,
// it prepares a staging directory with rendered files and returns that path.
// Otherwise returns the original source path in the repo.
func resolveSource(root, name string, secrets map[string]string) (string, error) {
	sourceDir := filepath.Join(root, name)
	if len(secrets) == 0 {
		return sourceDir, nil
	}

	needsRendering, err := render.NeedsRendering(sourceDir)
	if err != nil {
		return "", fmt.Errorf("store %q: check rendering: %w", name, err)
	}
	if !needsRendering {
		return sourceDir, nil
	}

	stagingBase, err := render.StagingDir(root)
	if err != nil {
		return "", fmt.Errorf("store %q: staging dir: %w", name, err)
	}

	stagingSource := filepath.Join(stagingBase, name)
	if _, err := render.PrepareStaging(sourceDir, stagingSource, secrets); err != nil {
		return "", fmt.Errorf("store %q: prepare staging: %w", name, err)
	}

	return stagingSource, nil
}

// StoreTargetWithSecrets is like StoreTarget but renders template files before linking.
func StoreTargetWithSecrets(root string, name string, te config.TargetEntry, secrets map[string]string) error {
	source, err := resolveSource(root, name, secrets)
	if err != nil {
		return err
	}

	target, err := config.ExpandHome(te.Target)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	return linkTarget(source, name, target, te)
}

// Store creates symlinks for a single store entry (all targets).
func Store(root string, name string, entry config.StoreEntry) error {
	return runStoreTargets(root, name, entry, "link", func(te config.TargetEntry) error {
		return StoreTarget(root, name, te)
	}, "store %q: %d target(s) failed")
}

// StoreWithSecrets is like Store but renders template files before linking.
func StoreWithSecrets(root string, name string, entry config.StoreEntry, secrets map[string]string) error {
	return runStoreTargets(root, name, entry, "link", func(te config.TargetEntry) error {
		return StoreTargetWithSecrets(root, name, te, secrets)
	}, "store %q: %d target(s) failed")
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
				if shouldUseFileMode(filepath.Join(root, name), te) {
					fmt.Printf("  %s -> %s (files)\n", name, te.Target)
				} else {
					fmt.Printf("  %s -> %s\n", name, te.Target)
				}
			}
		}
	}

	if len(errors) > 0 {
		fmt.Println()
		printErrors(errors)
		return fmt.Errorf("%d store(s) failed", len(errors))
	}

	return nil
}

// StoreAllWithSecrets is like StoreAll but renders template files before linking.
func StoreAllWithSecrets(root string, cfg *config.Config, secrets map[string]string) error {
	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	var errors []error
	for name, entry := range cfg.Stores {
		if err := StoreWithSecrets(root, name, entry, secrets); err != nil {
			errors = append(errors, err)
		} else {
			for _, te := range entry.ResolvedTargets() {
				if shouldUseFileMode(filepath.Join(root, name), te) {
					fmt.Printf("  %s -> %s (files)\n", name, te.Target)
				} else {
					fmt.Printf("  %s -> %s\n", name, te.Target)
				}
			}
		}
	}

	if len(errors) > 0 {
		fmt.Println()
		printErrors(errors)
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

	useFileMode := shouldUseFileMode(source, te)
	if !useFileMode {
		status, err := linker.Check(source, target)
		if err != nil {
			return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
		}

		switch status {
		case linker.StatusLinked, linker.StatusBroken:
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("store %q target %q: failed to remove symlink: %w", name, te.Target, err)
			}
		case linker.StatusMissing:
			// Nothing at target; no symlink to remove.
		case linker.StatusConflict:
			// Target exists but isn't managed by store; skip without error.
			fmt.Printf("  warning: %s is not a symlink managed by store, skipping unlink\n", target)
		}

		return nil
	}

	// File mode: resolve matches and unlink each file.
	files, err := resolveTargetMatches(source, te)
	if err != nil {
		return fmt.Errorf("store %q target %q: %w", name, te.Target, err)
	}

	var errors []error
	for _, rel := range files {
		src := filepath.Join(source, rel)
		tgt := filepath.Join(target, rel)
		status, err := linker.Check(src, tgt)
		if err != nil {
			errors = append(errors, fmt.Errorf("  %s: %w", rel, err))
			continue
		}

		switch status {
		case linker.StatusLinked, linker.StatusBroken:
			if err := os.Remove(tgt); err != nil {
				errors = append(errors, fmt.Errorf("  %s: %w", rel, err))
			}
		case linker.StatusMissing:
			// Nothing to do.
		case linker.StatusConflict:
			fmt.Printf("  warning: %s is not a symlink managed by store, skipping unlink\n", tgt)
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
	return runStoreTargets(root, name, entry, "unlink", func(te config.TargetEntry) error {
		return StoreRemoveTarget(root, name, te)
	}, "store %q: %d target(s) failed to unlink")
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
		printErrors(errors)
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
			Error: fmt.Errorf("no target configured -- did you mean `target: \"~\"`?"),
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

		if !shouldUseFileMode(source, te) {
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
		files, err := resolveTargetMatches(source, te)
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
