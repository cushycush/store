package main

import (
	"fmt"

	"github.com/cushycush/store/internal/config"
	storeops "github.com/cushycush/store/internal/store"
)

// printConflicts lists conflicts and what will happen to each file.
func printConflicts(conflicts []storeops.ConflictInfo) {
	fmt.Println("The following files conflict with store symlinks:")
	for _, conflict := range conflicts {
		kind := "file"
		if conflict.IsDir {
			kind = "directory"
		}
		fmt.Printf("  %s (%s -> will be moved to %s)\n", conflict.Target, kind, conflict.Source)
	}
}

func checkBackups(conflicts []storeops.ConflictInfo) error {
	backups, err := storeops.CollectBackups(conflicts)
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		return nil
	}

	fmt.Println("The following files in the store will be backed up (.bak):")
	for _, backup := range backups {
		fmt.Printf(" %s\n", backup)
	}
	fmt.Println()

	if !forceBackups && !promptYesNo("Proceed with creating backups?") {
		return fmt.Errorf("aborted due to backup conflicts")
	}

	return nil
}

// storeWithConflictResolution checks for conflicts, prompts the user to resolve
// them, then creates symlinks for all targets in the entry.
func storeWithConflictResolution(root, name string, entry config.StoreEntry, secrets map[string]string) error {
	conflicts, err := storeops.CollectConflicts(root, name, entry)
	if err != nil {
		return err
	}
	if err := resolveConflicts(conflicts); err != nil {
		return err
	}
	return storeops.StoreWithSecrets(root, name, entry, secrets)
}

// storeTargetWithConflictResolution checks for conflicts on a single target,
// prompts the user, resolves them, then creates symlinks.
func storeTargetWithConflictResolution(root, name string, targetEntry config.TargetEntry, secrets map[string]string) error {
	conflicts, err := storeops.CollectTargetConflicts(root, name, targetEntry)
	if err != nil {
		return err
	}
	if err := resolveConflicts(conflicts); err != nil {
		return err
	}
	return storeops.StoreTargetWithSecrets(root, name, targetEntry, secrets)
}

// storeAllWithConflictResolution checks for conflicts across all stores,
// prompts once, resolves, then creates all symlinks.
func storeAllWithConflictResolution(root string, cfg *config.Config, secrets map[string]string) error {
	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	var conflicts []storeops.ConflictInfo
	for name, entry := range cfg.Stores {
		entryConflicts, err := storeops.CollectConflicts(root, name, entry)
		if err != nil {
			return err
		}
		conflicts = append(conflicts, entryConflicts...)
	}

	if err := resolveConflicts(conflicts); err != nil {
		return err
	}
	return storeops.StoreAllWithSecrets(root, cfg, secrets)
}

func resolveConflicts(conflicts []storeops.ConflictInfo) error {
	if len(conflicts) == 0 {
		return nil
	}

	printConflicts(conflicts)
	fmt.Println()
	if !promptYesNo("Move these files into the store and create symlinks?") {
		return fmt.Errorf("aborted due to unresolved conflicts")
	}
	if err := checkBackups(conflicts); err != nil {
		return err
	}
	if err := storeops.ResolveConflicts(conflicts); err != nil {
		return err
	}
	fmt.Println()
	return nil
}
