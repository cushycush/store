package store

import (
	"fmt"
	"path/filepath"

	"github.com/cush/store/internal/config"
	"github.com/cush/store/internal/linker"
	"github.com/cush/store/internal/matcher"
)

// Store creates symlinks for a single store entry.
// In file mode (files/patterns specified), it creates per-file symlinks.
// Otherwise, it creates a single directory symlink.
func Store(root string, name string, entry config.StoreEntry) error {
	if entry.Target == "" {
		return nil // No target configured yet; skip silently.
	}

	source := filepath.Join(root, name)
	target, err := config.ExpandHome(entry.Target)
	if err != nil {
		return fmt.Errorf("store %q: %w", name, err)
	}

	if !entry.HasFileMode() {
		if err := linker.Link(source, target); err != nil {
			return fmt.Errorf("store %q: %w", name, err)
		}
		return nil
	}

	// File mode: resolve matches and link each file.
	files, err := matcher.Match(source, entry.Files, entry.Patterns)
	if err != nil {
		return fmt.Errorf("store %q: %w", name, err)
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
		return fmt.Errorf("store %q: %d file(s) failed", name, len(errors))
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
		} else if entry.Target != "" {
			if entry.HasFileMode() {
				fmt.Printf("  %s -> %s (files)\n", name, entry.Target)
			} else {
				fmt.Printf("  %s -> %s\n", name, entry.Target)
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

// StoreRemove removes symlinks for a single store.
func StoreRemove(root string, name string, entry config.StoreEntry) error {
	if entry.Target == "" {
		return nil
	}

	source := filepath.Join(root, name)
	target, err := config.ExpandHome(entry.Target)
	if err != nil {
		return fmt.Errorf("store %q: %w", name, err)
	}

	if !entry.HasFileMode() {
		if err := linker.Unlink(source, target); err != nil {
			return fmt.Errorf("store %q: %w", name, err)
		}
		return nil
	}

	// File mode: resolve matches and unlink each file.
	files, err := matcher.Match(source, entry.Files, entry.Patterns)
	if err != nil {
		return fmt.Errorf("store %q: %w", name, err)
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
		return fmt.Errorf("store %q: %d file(s) failed to unlink", name, len(errors))
	}

	return nil
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
		} else if entry.Target != "" {
			fmt.Printf("  removed %s (%s)\n", name, entry.Target)
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

// GetStatus checks the symlink status of a single store.
// For file-mode entries, it returns one StatusInfo per matched file.
func GetStatus(root string, name string, entry config.StoreEntry) []StatusInfo {
	if entry.Target == "" {
		return []StatusInfo{{
			Name:  name,
			Error: fmt.Errorf("no target configured"),
		}}
	}

	source := filepath.Join(root, name)
	target, err := config.ExpandHome(entry.Target)
	if err != nil {
		return []StatusInfo{{Name: name, Target: entry.Target, Error: err}}
	}

	if !entry.HasFileMode() {
		info := StatusInfo{
			Name:   name,
			Target: entry.Target,
		}
		status, err := linker.Check(source, target)
		if err != nil {
			info.Error = err
		} else {
			info.Status = status
		}
		return []StatusInfo{info}
	}

	// File mode: check each matched file.
	files, err := matcher.Match(source, entry.Files, entry.Patterns)
	if err != nil {
		return []StatusInfo{{Name: name, Target: entry.Target, Error: err}}
	}

	results := make([]StatusInfo, 0, len(files))
	for _, rel := range files {
		src := filepath.Join(source, rel)
		tgt := filepath.Join(target, rel)
		info := StatusInfo{
			Name:   name,
			File:   rel,
			Target: filepath.Join(entry.Target, rel),
		}
		status, err := linker.Check(src, tgt)
		if err != nil {
			info.Error = err
		} else {
			info.Status = status
		}
		results = append(results, info)
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
