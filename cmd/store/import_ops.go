package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/importer"
	"github.com/spf13/cobra"
)

func runInit(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if config.Exists(cwd) {
		return fmt.Errorf("%s already exists", config.ConfigPath(cwd))
	}

	cfg := &config.Config{Stores: make(map[string]config.StoreEntry)}
	if err := config.Save(cwd, cfg); err != nil {
		return err
	}

	fmt.Printf("Initialized store config at %s\n", config.ConfigPath(cwd))
	return nil
}

func runImport(scanDirs []string, dryRun bool) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	if len(scanDirs) == 0 {
		scanDirs = defaultImportScanDirs()
	}

	fmt.Printf("Scanning for symlinks pointing into %s...\n\n", root)

	links, err := importer.Scan(root, scanDirs)
	if err != nil {
		return err
	}
	links = filterImportLinks(links, cfg.Stores)
	if len(links) == 0 {
		fmt.Println("No new symlinks found")
		return nil
	}

	printImportLinks(root, links)
	entries := importer.ToConfig(links, root)

	if dryRun {
		fmt.Printf("\nDry run: would import %s as %s\n", pluralizeCount(len(links), "symlink", "symlinks"), pluralizeCount(len(entries), "store", "stores"))
		return nil
	}

	if !forceBackups {
		prompt := fmt.Sprintf("Import %s as %s?", pluralizeCount(len(links), "symlink", "symlinks"), pluralizeCount(len(entries), "store", "stores"))
		if !promptYesNo(prompt) {
			return fmt.Errorf("aborted")
		}
	}

	for name, entry := range entries {
		if _, exists := cfg.Stores[name]; exists {
			continue
		}
		cfg.Stores[name] = entry
	}

	if err := config.Save(root, cfg); err != nil {
		return err
	}

	configPath, err := filepath.Rel(root, config.ConfigPath(root))
	if err != nil {
		configPath = config.ConfigPath(root)
	}

	fmt.Printf("Imported %s to %s\n", pluralizeCount(len(entries), "store", "stores"), configPath)
	return nil
}

func defaultImportScanDirs() []string {
	return []string{"~", "~/.config", "~/.local/share", "~/.local/bin"}
}

func filterImportLinks(links []importer.DiscoveredLink, existing map[string]config.StoreEntry) []importer.DiscoveredLink {
	filtered := make([]importer.DiscoveredLink, 0, len(links))
	for _, link := range links {
		if _, exists := existing[link.StoreName]; exists {
			continue
		}
		filtered = append(filtered, link)
	}
	return filtered
}

func printImportLinks(root string, links []importer.DiscoveredLink) {
	fmt.Println("Found:")

	nameWidth := len("store")
	mappingWidth := 0
	for _, link := range links {
		if len(link.StoreName) > nameWidth {
			nameWidth = len(link.StoreName)
		}
		mapping := fmt.Sprintf("%s -> %s", formatImportPath(link.Target), importSourceDisplay(root, link))
		if len(mapping) > mappingWidth {
			mappingWidth = len(mapping)
		}
	}

	for _, link := range links {
		kind := "file"
		if link.File == "" {
			kind = "whole directory"
		}

		mapping := fmt.Sprintf("%s -> %s", formatImportPath(link.Target), importSourceDisplay(root, link))
		fmt.Printf("  %-*s %-*s (%s)\n", nameWidth, link.StoreName, mappingWidth, mapping, kind)
	}
}

func formatImportPath(path string) string {
	cleaned := filepath.Clean(path)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return cleaned
	}

	home = filepath.Clean(home)
	if cleaned == home {
		return "~"
	}

	prefix := home + string(os.PathSeparator)
	if suffix, ok := strings.CutPrefix(cleaned, prefix); ok {
		return filepath.Join("~", suffix)
	}

	return cleaned
}

func importSourceDisplay(root string, link importer.DiscoveredLink) string {
	rel, err := filepath.Rel(root, link.Source)
	if err != nil {
		rel = link.Source
	}

	rel = filepath.ToSlash(filepath.Clean(rel))
	if link.File == "" && !strings.HasSuffix(rel, "/") {
		return rel + "/"
	}
	return rel
}
