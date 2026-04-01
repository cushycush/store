package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cush/store/internal/config"
	"github.com/cush/store/internal/linker"
	storeops "github.com/cush/store/internal/store"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "store",
		Short: "A simpler alternative to GNU stow",
		Long:  "store manages symlinks for your dotfiles without requiring mirrored directory structures.",
		RunE:  runStoreAll,
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new store config",
		Long:  "Creates a .store/config.yaml file in the current directory.",
		RunE:  runInit,
	}

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a store and create its symlink",
		Long:  "Prompts for the target path, saves to config, and creates the directory symlink.",
		Args:  cobra.ExactArgs(1),
		RunE:  runAdd,
	}

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

	rootCmd.AddCommand(initCmd, addCmd, removeCmd, removeAllCmd, statusCmd)

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

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	// Ensure the store directory exists, creating it if needed.
	storePath := root + "/" + name
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

	// Prompt for target path.
	fmt.Printf("Where should %q be symlinked to?\n", name)
	fmt.Print("Target path: ")

	reader := bufio.NewReader(os.Stdin)
	target, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	target = strings.TrimSpace(target)

	if target == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	// Load existing config (or create fresh if somehow missing).
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	// Save the entry to config.
	cfg.Stores[name] = config.StoreEntry{Target: target}
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	// Create the symlink.
	if err := storeops.Store(root, name, cfg.Stores[name]); err != nil {
		return err
	}

	fmt.Printf("  %s -> %s\n", name, target)
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

		info := storeops.GetStatus(root, name, entry)
		printStatus(info)
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
		fmt.Printf("  %-20s %s  (error: %s)\n", info.Name, info.Target, info.Error)
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

	fmt.Printf("  %-20s %-10s %s\n", info.Name, indicator, info.Target)
}
