package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/hooks"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/platform"
	storeops "github.com/cushycush/store/internal/store"
	"github.com/cushycush/store/internal/ui"
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
		fmt.Printf("%s %s\n", ui.Green("Created directory"), ui.TargetPath(storePath))
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
		fmt.Printf("%s %s %s\n", ui.Green("Added"), ui.StoreName(name), ui.Dim("to config (no target set)"))
		return nil
	}

	rc, err := buildRenderContext(root, cfg, name)
	if err != nil {
		return err
	}
	if err := storeWithConflictResolution(root, name, entry, rc); err != nil {
		return err
	}

	printStoredTarget(name, target, entry.HasFileMode())
	return nil
}

type modifyOps struct {
	addFiles      []string
	removeFiles   []string
	addPatterns   []string
	removePatterns []string
}

func runModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool, ops modifyOps) error {
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
		fmt.Println(ui.Dim(fmt.Sprintf("  warning: failed to remove old symlinks: %s", err)))
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
	if clearFiles {
		entry.Files = nil
	}
	if cmd.Flags().Changed("files") {
		entry.Files = files
	}
	entry.Files = applyListOps(entry.Files, ops.addFiles, ops.removeFiles)
	if clearPatterns {
		entry.Patterns = nil
	}
	if cmd.Flags().Changed("patterns") {
		entry.Patterns = patterns
	}
	entry.Patterns = applyListOps(entry.Patterns, ops.addPatterns, ops.removePatterns)

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	if entry.Target == "" {
		fmt.Printf("%s %s %s\n", ui.Green("Modified"), ui.StoreName(name), ui.Dim("(no target set)"))
		return nil
	}

	rc, err := buildRenderContext(root, cfg, name)
	if err != nil {
		return err
	}
	if err := storeWithConflictResolution(root, name, entry, rc); err != nil {
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

	rc, err := buildRenderContext(root, cfg, names...)
	if err != nil {
		return err
	}

	fmt.Println(ui.Bold("Storing all stores:"))
	err = storeAllWithConflictResolution(root, cfg, rc)

	if err := hooks.RunGlobal(root, "post-store", "link"); err != nil {
		fmt.Println(ui.Dim(fmt.Sprintf("  warning: %s", err)))
	}

	return err
}

// previewRemove prints what runRemove would do without touching config or filesystem.
func previewRemove(name string) error {
	_, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}
	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config", name)
	}
	targets := entry.ResolvedTargets()
	fmt.Printf("%s would remove store %s (%d target(s))\n", ui.Dim("[dry-run]"), ui.StoreName(name), len(targets))
	for _, t := range targets {
		fmt.Printf("  - unlink %s\n", ui.TargetPath(t.Target))
	}
	return nil
}

// previewRemoveAll prints what runRemoveAll would do without touching anything.
func previewRemoveAll() error {
	_, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}
	if len(cfg.Stores) == 0 {
		fmt.Println(ui.Dim("[dry-run]") + " no stores configured")
		return nil
	}
	stores := selectStores(cfg.Stores, onlyStores)
	stores = filterStoresByPlatform(stores, platform.Detect())
	fmt.Printf("%s would remove %d store(s):\n", ui.Dim("[dry-run]"), len(stores))
	for name, entry := range stores {
		fmt.Printf("  - %s (%s)\n", ui.StoreName(name), ui.TargetPath(entry.Target))
	}
	return nil
}

// previewTargetAdd prints the target that would be added to the named store.
func previewTargetAdd(name, target string, files, patterns []string) error {
	_, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Stores[name]; !ok {
		return fmt.Errorf("store %q not found in config", name)
	}
	fmt.Printf("%s would add target to store %s:\n", ui.Dim("[dry-run]"), ui.StoreName(name))
	fmt.Printf("  target:   %s\n", target)
	if len(files) > 0 {
		fmt.Printf("  files:    %v\n", files)
	}
	if len(patterns) > 0 {
		fmt.Printf("  patterns: %v\n", patterns)
	}
	return nil
}

// previewTargetRemove prints the target that would be removed from a store.
func previewTargetRemove(name, target string) error {
	_, cfg, err := findRootAndConfig()
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
	found := false
	for _, t := range entry.ResolvedTargets() {
		expandedExisting, err := expandTargetPath(t.Target)
		if err != nil {
			return err
		}
		if expandedExisting == expandedTarget {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("store %q has no target %q", name, target)
	}
	fmt.Printf("%s would remove target %s from store %s\n", ui.Dim("[dry-run]"), ui.TargetPath(target), ui.StoreName(name))
	return nil
}

// previewTargetModify prints the would-be post-modification target entry.
func previewTargetModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool) error {
	_, cfg, err := findRootAndConfig()
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
	var found *config.TargetEntry
	for i := range entry.Targets {
		expandedExisting, err := expandTargetPath(entry.Targets[i].Target)
		if err != nil {
			return err
		}
		if expandedExisting == expandedTarget {
			found = &entry.Targets[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("store %q has no target %q", name, target)
	}
	if cmd.Flags().Changed("files") {
		found.Files = files
	}
	if clearFiles {
		found.Files = nil
	}
	if cmd.Flags().Changed("patterns") {
		found.Patterns = patterns
	}
	if clearPatterns {
		found.Patterns = nil
	}
	fmt.Printf("%s would modify target %s on store %s:\n", ui.Dim("[dry-run]"), ui.TargetPath(target), ui.StoreName(name))
	fmt.Printf("  files:    %v\n", found.Files)
	fmt.Printf("  patterns: %v\n", found.Patterns)
	return nil
}

// previewModify prints the would-be new entry after applying the provided modifications.
func previewModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool) error {
	_, cfg, err := findRootAndConfig()
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
	if cmd.Flags().Changed("target") {
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
	fmt.Printf("%s would modify store %s:\n", ui.Dim("[dry-run]"), ui.StoreName(name))
	fmt.Printf("  target:   %s\n", entry.Target)
	fmt.Printf("  files:    %v\n", entry.Files)
	fmt.Printf("  patterns: %v\n", entry.Patterns)
	return nil
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
		fmt.Printf("%s store %s (%s)\n", ui.Green("Removed"), ui.StoreName(name), ui.TargetPath(targets[0].Target))
		return nil
	}

	fmt.Printf("%s store %s (%s targets)\n", ui.Green("Removed"), ui.StoreName(name), ui.Bold(fmt.Sprintf("%d", len(targets))))
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

	fmt.Println(ui.Bold("Removing all stores:"))
	var errors []error
	for name, entry := range storesToRemove {
		if err := storeops.StoreRemove(root, name, entry); err != nil {
			errors = append(errors, err)
			continue
		}

		delete(cfg.Stores, name)
		fmt.Printf("  %s %s (%s)\n", ui.Green("Removed"), ui.StoreName(name), ui.TargetPath(entry.Target))
	}

	if err := config.Save(root, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if len(errors) > 0 {
		fmt.Println()
		for _, err := range errors {
			fmt.Println(ui.BoldRed(fmt.Sprintf("  error: %s", err)))
		}
		return fmt.Errorf("%d store(s) failed", len(errors))
	}

	if err := hooks.RunGlobal(root, "post-remove", "unlink"); err != nil {
		fmt.Println(ui.Dim(fmt.Sprintf("  warning: %s", err)))
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

		results := storeops.GetStatus(root, name, entry)
		for _, info := range results {
			printStatus(info)
		}
		printStatusSummary(results)
		return nil
	}

	cfg.Stores = filterStoresByPlatform(cfg.Stores, platform.Detect())
	if len(cfg.Stores) == 0 {
		fmt.Println(ui.Dim("No stores defined in config."))
		return nil
	}

	results := storeops.GetStatusAll(root, cfg)
	for _, info := range results {
		printStatus(info)
	}
	printStatusSummary(results)
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

	rc, err := buildRenderContext(root, cfg, name)
	if err != nil {
		return err
	}
	if err := storeTargetWithConflictResolution(root, name, newTarget, rc); err != nil {
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
		fmt.Println(ui.Dim(fmt.Sprintf("  warning: failed to remove symlinks: %s", err)))
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

	fmt.Printf("  %s target %s from %s\n", ui.Green("Removed"), ui.TargetPath(target), ui.StoreName(name))
	return nil
}

func runTargetModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool, ops modifyOps) error {
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
		fmt.Println(ui.Dim(fmt.Sprintf("  warning: failed to remove old symlinks: %s", err)))
	}

	targetEntry := &entry.Targets[found]
	if clearFiles {
		targetEntry.Files = nil
	}
	if cmd.Flags().Changed("files") {
		targetEntry.Files = files
	}
	targetEntry.Files = applyListOps(targetEntry.Files, ops.addFiles, ops.removeFiles)
	if clearPatterns {
		targetEntry.Patterns = nil
	}
	if cmd.Flags().Changed("patterns") {
		targetEntry.Patterns = patterns
	}
	targetEntry.Patterns = applyListOps(targetEntry.Patterns, ops.addPatterns, ops.removePatterns)

	entry.MigrateToSingleTarget()

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	rc, err := buildRenderContext(root, cfg, name)
	if err != nil {
		return err
	}

	for _, resolved := range entry.ResolvedTargets() {
		if resolved.Target != target {
			continue
		}
		if err := storeTargetWithConflictResolution(root, name, resolved, rc); err != nil {
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
			fmt.Printf("  %s %s %s  %s\n", ui.StoreName(fmt.Sprintf("%-20s", info.Name)), ui.FileName(fmt.Sprintf("%-20s", info.File)), ui.TargetPath(info.Target), ui.BoldRed(fmt.Sprintf("(error: %s)", info.Error)))
			return
		}
		fmt.Printf("  %s %s  %s\n", ui.StoreName(fmt.Sprintf("%-20s", info.Name)), ui.TargetPath(info.Target), ui.BoldRed(fmt.Sprintf("(error: %s)", info.Error)))
		return
	}

	indicator := statusIndicator(info.Status)
	if info.File != "" {
		fmt.Printf("  %s %s %s %s\n", ui.StoreName(fmt.Sprintf("%-20s", info.Name)), ui.FileName(fmt.Sprintf("%-20s", info.File)), indicator, ui.TargetPath(info.Target))
		return
	}
	fmt.Printf("  %s %s %s\n", ui.StoreName(fmt.Sprintf("%-20s", info.Name)), indicator, ui.TargetPath(info.Target))
}

func statusIndicator(status linker.Status) string {
	switch status {
	case linker.StatusLinked:
		return ui.StatusLinked() + strings.Repeat(" ", 10-len("[linked]"))
	case linker.StatusMissing:
		return ui.StatusMissing() + strings.Repeat(" ", 10-len("[missing]"))
	case linker.StatusConflict:
		return ui.StatusConflict() + strings.Repeat(" ", 10-len("[conflict]"))
	case linker.StatusBroken:
		return ui.StatusBroken() + strings.Repeat(" ", 10-len("[broken]"))
	case linker.StatusDrift:
		return ui.StatusDrift() + strings.Repeat(" ", 10-len("[drift]"))
	default:
		return ""
	}
}

func printStatusSummary(results []storeops.StatusInfo) {
	counts := map[linker.Status]int{}
	var errCount int
	for _, info := range results {
		if info.Error != nil {
			errCount++
		} else {
			counts[info.Status]++
		}
	}

	parts := []string{
		fmt.Sprintf("%s linked", ui.Bold(fmt.Sprintf("%d", counts[linker.StatusLinked]))),
	}
	if n := counts[linker.StatusMissing]; n > 0 {
		parts = append(parts, fmt.Sprintf("%s missing", ui.Bold(fmt.Sprintf("%d", n))))
	}
	if n := counts[linker.StatusConflict]; n > 0 {
		parts = append(parts, fmt.Sprintf("%s conflict", ui.Bold(fmt.Sprintf("%d", n))))
	}
	if n := counts[linker.StatusBroken]; n > 0 {
		parts = append(parts, fmt.Sprintf("%s broken", ui.Bold(fmt.Sprintf("%d", n))))
	}
	if n := counts[linker.StatusDrift]; n > 0 {
		parts = append(parts, fmt.Sprintf("%s drift", ui.Bold(fmt.Sprintf("%d", n))))
	}
	if errCount > 0 {
		parts = append(parts, fmt.Sprintf("%s error", ui.Bold(fmt.Sprintf("%d", errCount))))
	}

	fmt.Printf("\n%s\n", strings.Join(parts, ", "))
}
