package store

import (
	"fmt"
	"path/filepath"

	"github.com/cush/store/internal/config"
	"github.com/cush/store/internal/linker"
)

// Store creates the directory symlink for a single store.
func Store(root string, name string, entry config.StoreEntry) error {
	source := filepath.Join(root, name)
	target, err := config.ExpandHome(entry.Target)
	if err != nil {
		return fmt.Errorf("store %q: %w", name, err)
	}

	if err := linker.Link(source, target); err != nil {
		return fmt.Errorf("store %q: %w", name, err)
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
			fmt.Printf("  %s -> %s\n", name, entry.Target)
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

// StoreRemove removes the symlink for a single store.
func StoreRemove(root string, name string, entry config.StoreEntry) error {
	source := filepath.Join(root, name)
	target, err := config.ExpandHome(entry.Target)
	if err != nil {
		return fmt.Errorf("store %q: %w", name, err)
	}

	if err := linker.Unlink(source, target); err != nil {
		return fmt.Errorf("store %q: %w", name, err)
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
		} else {
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

// StatusInfo holds the status of a single store.
type StatusInfo struct {
	Name   string
	Target string
	Status linker.Status
	Error  error
}

// GetStatus checks the symlink status of a single store.
func GetStatus(root string, name string, entry config.StoreEntry) StatusInfo {
	info := StatusInfo{
		Name:   name,
		Target: entry.Target,
	}

	source := filepath.Join(root, name)
	target, err := config.ExpandHome(entry.Target)
	if err != nil {
		info.Error = err
		return info
	}

	status, err := linker.Check(source, target)
	if err != nil {
		info.Error = err
		return info
	}

	info.Status = status
	return info
}

// GetStatusAll checks the symlink status of all stores.
func GetStatusAll(root string, cfg *config.Config) []StatusInfo {
	var results []StatusInfo
	for name, entry := range cfg.Stores {
		results = append(results, GetStatus(root, name, entry))
	}
	return results
}
