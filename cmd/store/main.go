package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cush/store/internal/config"
	"github.com/cush/store/internal/linker"
	storeops "github.com/cush/store/internal/store"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "store",
		Short:   "A simpler alternative to GNU stow",
		Long:    "store manages symlinks for your dotfiles without requiring mirrored directory structures.",
		Version: version,
		RunE:    runStoreAll,
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new store config",
		Long:  "Creates a .store/config.yaml file in the current directory.",
		RunE:  runInit,
	}

	// --- add command ---
	var addTarget string
	var addFiles []string
	var addPatterns []string

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a store to config and create its symlinks",
		Long: `Adds a new store entry to config. Use flags to set the target path,
explicit files, and/or glob patterns for file-level symlinks.

Without --target, the entry is saved to config but no symlinks are created.
Without --files or --patterns, the entire directory is symlinked to the target.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(args[0], addTarget, addFiles, addPatterns)
		},
	}
	addCmd.Flags().StringVarP(&addTarget, "target", "t", "", "target path for the symlink")
	addCmd.Flags().StringArrayVarP(&addFiles, "files", "f", nil, "explicit files to symlink (repeatable)")
	addCmd.Flags().StringArrayVarP(&addPatterns, "patterns", "p", nil, "glob patterns to match files (repeatable, supports **)")

	// --- modify command ---
	var modTarget string
	var modFiles []string
	var modPatterns []string
	var modClearFiles bool
	var modClearPatterns bool

	modifyCmd := &cobra.Command{
		Use:   "modify <name>",
		Short: "Modify an existing store entry",
		Long: `Updates fields on an existing store entry. Each provided flag replaces
the entire field. Use --clear-files or --clear-patterns to remove those fields.

The old symlinks are removed before applying changes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModify(cmd, args[0], modTarget, modFiles, modPatterns, modClearFiles, modClearPatterns)
		},
	}
	modifyCmd.Flags().StringVarP(&modTarget, "target", "t", "", "new target path")
	modifyCmd.Flags().StringArrayVarP(&modFiles, "files", "f", nil, "replace file list (repeatable)")
	modifyCmd.Flags().StringArrayVarP(&modPatterns, "patterns", "p", nil, "replace pattern list (repeatable)")
	modifyCmd.Flags().BoolVar(&modClearFiles, "clear-files", false, "remove all files from the entry")
	modifyCmd.Flags().BoolVar(&modClearPatterns, "clear-patterns", false, "remove all patterns from the entry")

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a store's symlink",
		Long:  "Removes the symlink for the named store and deletes its config entry.",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}

	removeAllCmd := &cobra.Command{
		Use:   "removeall",
		Short: "Remove all store symlinks",
		Long:  "Removes symlinks and config entries for all stores defined in the config.",
		RunE:  runRemoveAll,
	}

	statusCmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show symlink status",
		Long:  "Shows the symlink state for one or all stores.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runStatus,
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("store version %s\n", version)
		},
	}

	rootCmd.AddCommand(initCmd, addCmd, modifyCmd, removeCmd, removeAllCmd, statusCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if config.Exists(cwd) {
		return fmt.Errorf("%s already exists", config.ConfigPath(cwd))
	}

	cfg := &config.Config{
		Stores: make(map[string]config.StoreEntry),
	}

	if err := config.Save(cwd, cfg); err != nil {
		return err
	}

	fmt.Printf("Initialized store config at %s\n", config.ConfigPath(cwd))
	return nil
}

func runAdd(name, target string, files, patterns []string) error {
	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	if _, exists := cfg.Stores[name]; exists {
		return fmt.Errorf("store %q already exists (use 'store modify' to update it)", name)
	}

	// Ensure the store directory exists, creating it if needed.
	storePath := filepath.Join(root, name)
	fi, err := os.Stat(storePath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(storePath, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", storePath, err)
		}
		fmt.Printf("Created directory %s\n", storePath)
	} else if err != nil {
		return fmt.Errorf("failed to stat %s: %w", storePath, err)
	} else if !fi.IsDir() {
		return fmt.Errorf("%q is not a directory", name)
	}

	// Resolve target to absolute path; keep ~/... as-is for portability.
	if target != "" {
		expanded, err := config.ExpandHome(target)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(expanded) {
			target, err = filepath.Abs(target)
			if err != nil {
				return fmt.Errorf("failed to resolve target path: %w", err)
			}
		}
	}

	entry := config.StoreEntry{
		Target:   target,
		Files:    files,
		Patterns: patterns,
	}

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	// Create symlinks if a target is configured.
	if target != "" {
		if err := storeops.Store(root, name, entry); err != nil {
			return err
		}
		if entry.HasFileMode() {
			fmt.Printf("  %s -> %s (files)\n", name, target)
		} else {
			fmt.Printf("  %s -> %s\n", name, target)
		}
	} else {
		fmt.Printf("Added %s to config (no target set)\n", name)
	}

	return nil
}

func runModify(cmd *cobra.Command, name, target string, files, patterns []string, clearFiles, clearPatterns bool) error {
	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	entry, ok := cfg.Stores[name]
	if !ok {
		return fmt.Errorf("store %q not found in config", name)
	}

	// Remove old symlinks before modifying.
	if err := storeops.StoreRemove(root, name, entry); err != nil {
		fmt.Printf("  warning: failed to remove old symlinks: %s\n", err)
	}

	// Apply modifications — each flag replaces the entire field.
	if cmd.Flags().Changed("target") {
		if target != "" {
			expanded, err := config.ExpandHome(target)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(expanded) {
				target, err = filepath.Abs(target)
				if err != nil {
					return fmt.Errorf("failed to resolve target path: %w", err)
				}
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

	// Re-create symlinks with updated config.
	if entry.Target != "" {
		if err := storeops.Store(root, name, entry); err != nil {
			return err
		}
		if entry.HasFileMode() {
			fmt.Printf("  %s -> %s (files)\n", name, entry.Target)
		} else {
			fmt.Printf("  %s -> %s\n", name, entry.Target)
		}
	} else {
		fmt.Printf("Modified %s (no target set)\n", name)
	}

	return nil
}

func runStoreAll(cmd *cobra.Command, args []string) error {
	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	fmt.Println("Storing all stores:")
	return storeops.StoreAll(root, cfg)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
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

	fmt.Printf("Removed store %s (%s)\n", name, entry.Target)
	return nil
}

func runRemoveAll(cmd *cobra.Command, args []string) error {
	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	fmt.Println("Removing all stores:")
	var errors []error
	for name, entry := range cfg.Stores {
		if err := storeops.StoreRemove(root, name, entry); err != nil {
			errors = append(errors, err)
		} else {
			delete(cfg.Stores, name)
			fmt.Printf("  removed %s (%s)\n", name, entry.Target)
		}
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

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		name := args[0]
		entry, ok := cfg.Stores[name]
		if !ok {
			return fmt.Errorf("store %q not found in config", name)
		}

		infos := storeops.GetStatus(root, name, entry)
		for _, info := range infos {
			printStatus(info)
		}
		return nil
	}

	// Show all stores.
	if len(cfg.Stores) == 0 {
		fmt.Println("No stores defined in config.")
		return nil
	}

	results := storeops.GetStatusAll(root, cfg)
	for _, info := range results {
		printStatus(info)
	}
	return nil
}

func printStatus(info storeops.StatusInfo) {
	if info.Error != nil {
		if info.File != "" {
			fmt.Printf("  %-20s %-20s %s  (error: %s)\n", info.Name, info.File, info.Target, info.Error)
		} else {
			fmt.Printf("  %-20s %s  (error: %s)\n", info.Name, info.Target, info.Error)
		}
		return
	}

	var indicator string
	switch info.Status {
	case linker.StatusLinked:
		indicator = "[linked]"
	case linker.StatusMissing:
		indicator = "[missing]"
	case linker.StatusConflict:
		indicator = "[conflict]"
	case linker.StatusBroken:
		indicator = "[broken]"
	}

	if info.File != "" {
		fmt.Printf("  %-20s %-20s %-10s %s\n", info.Name, info.File, indicator, info.Target)
	} else {
		fmt.Printf("  %-20s %-10s %s\n", info.Name, indicator, info.Target)
	}
}
