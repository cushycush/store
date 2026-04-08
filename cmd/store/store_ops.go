package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/hooks"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/platform"
	storeops "github.com/cushycush/store/internal/store"
	"github.com/spf13/cobra"
)

func runAdd(name, target string, files, patterns []string) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	if _, exists := cfg.Stores[name]; exists {
		return fmt.Errorf("store %q already exists (use 'store modify' to update it)", name)
	}

	storePath := filepath.Join(root, name)
	info, err := os.Stat(storePath)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(storePath, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", storePath, err)
		}
		fmt.Printf("Created directory %s\n", storePath)
	case err != nil:
		return fmt.Errorf("failed to stat %s: %w", storePath, err)
	case !info.IsDir():
		return fmt.Errorf("%q is not a directory", name)
	}

	if target != "" {
		target, err = resolveTargetPath(target)
		if err != nil {
			return err
		}
	}

	entry := config.StoreEntry{Target: target, Files: files, Patterns: patterns}
	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	if target == "" {
		fmt.Printf("Added %s to config (no target set)\n", name)
		return nil
	}

	secretMap, err := loadSecretsIfNeeded(root, name)
	if err != nil {
		return err
	}
	if err := storeWithConflictResolution(root, name, entry, secretMap); err != nil {
		return err
	}

	printStoredTarget(name, target, entry.HasFileMode())
	return nil
}

func runModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config", name)
	}
	if entry.IsMultiTarget() {
		return fmt.Errorf("store %q uses multiple targets; use 'store target modify' instead", name)
	}

	if err := storeops.StoreRemove(root, name, entry); err != nil {
		fmt.Printf("  warning: failed to remove old symlinks: %s\n", err)
	}

	if cmd.Flags().Changed("target") {
		if target != "" {
			target, err = resolveTargetPath(target)
			if err != nil {
				return err
			}
		}
		entry.Target = target
	}
	if cmd.Flags().Changed("files") {
		entry.Files = files
	}
	if clearFiles {
		entry.Files = nil
	}
	if cmd.Flags().Changed("patterns") {
		entry.Patterns = patterns
	}
	if clearPatterns {
		entry.Patterns = nil
	}

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	if entry.Target == "" {
		fmt.Printf("Modified %s (no target set)\n", name)
		return nil
	}

	secretMap, err := loadSecretsIfNeeded(root, name)
	if err != nil {
		return err
	}
	if err := storeWithConflictResolution(root, name, entry, secretMap); err != nil {
		return err
	}

	printStoredTarget(name, entry.Target, entry.HasFileMode())
	return nil
}

func runStoreAll(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	cfg.Stores = selectStores(cfg.Stores, onlyStores)
	selectedStores := cfg.Stores
	filteredStores := filterStoresByPlatform(selectedStores, platform.Detect())
	printPlatformSkippedStores(selectedStores, filteredStores)
	cfg.Stores = filteredStores
	if len(selectedStores) > 0 && len(cfg.Stores) == 0 {
		return nil
	}

	if err := hooks.RunGlobal(root, "pre-store", "link"); err != nil {
		return err
	}

	names := make([]string, 0, len(cfg.Stores))
	for name := range cfg.Stores {
		names = append(names, name)
	}

	secretMap, err := loadSecretsIfNeeded(root, names...)
	if err != nil {
		return err
	}

	fmt.Println("Storing all stores:")
	err = storeAllWithConflictResolution(root, cfg, secretMap)

	if err := hooks.RunGlobal(root, "post-store", "link"); err != nil {
		fmt.Printf("  warning: %s\n", err)
	}

	return err
}

func runRemove(cmd *cobra.Command, args []string) error {
	_ = cmd

	name := args[0]
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config", name)
	}

	if err := storeops.StoreRemove(root, name, entry); err != nil {
		return err
	}

	delete(cfg.Stores, name)
	if err := config.Save(root, cfg); err != nil {
		return fmt.Errorf("failed to remove config entry: %w", err)
	}

	targets := entry.ResolvedTargets()
	if len(targets) == 1 {
		fmt.Printf("Removed store %s (%s)\n", name, targets[0].Target)
		return nil
	}

	fmt.Printf("Removed store %s (%d targets)\n", name, len(targets))
	return nil
}

func runRemoveAll(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}
	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	storesToRemove := selectStores(cfg.Stores, onlyStores)
	storesToRemove = filterStoresByPlatform(storesToRemove, platform.Detect())
	if len(storesToRemove) == 0 && (len(cfg.Stores) > 0 || len(onlyStores) > 0) {
		return nil
	}

	if err := hooks.RunGlobal(root, "pre-remove", "unlink"); err != nil {
		return err
	}

	fmt.Println("Removing all stores:")
	var errors []error
	for name, entry := range storesToRemove {
		if err := storeops.StoreRemove(root, name, entry); err != nil {
			errors = append(errors, err)
			continue
		}

		delete(cfg.Stores, name)
		fmt.Printf("  removed %s (%s)\n", name, entry.Target)
	}

	if err := config.Save(root, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if len(errors) > 0 {
		fmt.Println()
		for _, err := range errors {
			fmt.Printf("  error: %s\n", err)
		}
		return fmt.Errorf("%d store(s) failed", len(errors))
	}

	if err := hooks.RunGlobal(root, "post-remove", "unlink"); err != nil {
		fmt.Printf("  warning: %s\n", err)
	}

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	_ = cmd

	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		name := args[0]
		entry, ok := cfg.Stores[name]
		if !ok {
			return fmt.Errorf("store %q not found in config", name)
		}

		for _, info := range storeops.GetStatus(root, name, entry) {
			printStatus(info)
		}
		return nil
	}

	cfg.Stores = filterStoresByPlatform(cfg.Stores, platform.Detect())
	if len(cfg.Stores) == 0 {
		fmt.Println("No stores defined in config.")
		return nil
	}

	for _, info := range storeops.GetStatusAll(root, cfg) {
		printStatus(info)
	}
	return nil
}

func runTargetAdd(name, target string, files, patterns []string) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config (use 'store add' to create it first)", name)
	}

	target, err = resolveTargetPath(target)
	if err != nil {
		return err
	}

	expandedTarget, err := expandTargetPath(target)
	if err != nil {
		return err
	}
	for _, targetEntry := range entry.ResolvedTargets() {
		expandedExisting, err := expandTargetPath(targetEntry.Target)
		if err != nil {
			return err
		}
		if expandedExisting == expandedTarget {
			return fmt.Errorf("store %q already has a target %q", name, targetEntry.Target)
		}
	}

	entry.MigrateToMultiTarget()

	newTarget := config.TargetEntry{Target: target, Files: files, Patterns: patterns}
	entry.Targets = append(entry.Targets, newTarget)

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	secretMap, err := loadSecretsIfNeeded(root, name)
	if err != nil {
		return err
	}
	if err := storeTargetWithConflictResolution(root, name, newTarget, secretMap); err != nil {
		return err
	}

	printStoredTarget(name, target, newTarget.HasFileMode())
	return nil
}

func runTargetRemove(name, target string) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config", name)
	}

	expandedTarget, err := expandTargetPath(target)
	if err != nil {
		return err
	}

	entry.MigrateToMultiTarget()

	found := -1
	for i, targetEntry := range entry.Targets {
		expandedExisting, err := expandTargetPath(targetEntry.Target)
		if err != nil {
			return err
		}
		if expandedExisting == expandedTarget {
			found = i
			break
		}
	}
	if found == -1 {
		return fmt.Errorf("store %q has no target %q", name, target)
	}

	if err := storeops.StoreRemoveTarget(root, name, entry.Targets[found]); err != nil {
		fmt.Printf("  warning: failed to remove symlinks: %s\n", err)
	}

	entry.Targets = append(entry.Targets[:found], entry.Targets[found+1:]...)
	if len(entry.Targets) == 0 {
		entry.Target = ""
		entry.Files = nil
		entry.Patterns = nil
		entry.Targets = nil
	} else {
		entry.MigrateToSingleTarget()
	}

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	fmt.Printf("  removed target %s from %s\n", target, name)
	return nil
}

func runTargetModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config", name)
	}

	expandedTarget, err := expandTargetPath(target)
	if err != nil {
		return err
	}

	entry.MigrateToMultiTarget()

	found := -1
	for i, targetEntry := range entry.Targets {
		expandedExisting, err := expandTargetPath(targetEntry.Target)
		if err != nil {
			return err
		}
		if expandedExisting == expandedTarget {
			found = i
			break
		}
	}
	if found == -1 {
		return fmt.Errorf("store %q has no target %q", name, target)
	}

	if err := storeops.StoreRemoveTarget(root, name, entry.Targets[found]); err != nil {
		fmt.Printf("  warning: failed to remove old symlinks: %s\n", err)
	}

	targetEntry := &entry.Targets[found]
	if cmd.Flags().Changed("files") {
		targetEntry.Files = files
	}
	if clearFiles {
		targetEntry.Files = nil
	}
	if cmd.Flags().Changed("patterns") {
		targetEntry.Patterns = patterns
	}
	if clearPatterns {
		targetEntry.Patterns = nil
	}

	entry.MigrateToSingleTarget()

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	secretMap, err := loadSecretsIfNeeded(root, name)
	if err != nil {
		return err
	}

	for _, resolved := range entry.ResolvedTargets() {
		if resolved.Target != target {
			continue
		}
		if err := storeTargetWithConflictResolution(root, name, resolved, secretMap); err != nil {
			return err
		}
		printStoredTarget(name, target, resolved.HasFileMode())
		break
	}

	return nil
}

func printStatus(info storeops.StatusInfo) {
	if info.Error != nil {
		if info.File != "" {
			fmt.Printf("  %-20s %-20s %s  (error: %s)\n", info.Name, info.File, info.Target, info.Error)
			return
		}
		fmt.Printf("  %-20s %s  (error: %s)\n", info.Name, info.Target, info.Error)
		return
	}

	indicator := statusIndicator(info.Status)
	if info.File != "" {
		fmt.Printf("  %-20s %-20s %-10s %s\n", info.Name, info.File, indicator, info.Target)
		return
	}
	fmt.Printf("  %-20s %-10s %s\n", info.Name, indicator, info.Target)
}

func statusIndicator(status linker.Status) string {
	switch status {
	case linker.StatusLinked:
		return "[linked]"
	case linker.StatusMissing:
		return "[missing]"
	case linker.StatusConflict:
		return "[conflict]"
	case linker.StatusBroken:
		return "[broken]"
	default:
		return ""
	}
}
