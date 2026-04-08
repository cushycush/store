package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/platform"
	"github.com/cushycush/store/internal/render"
	"github.com/cushycush/store/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func findRootAndConfig() (string, *config.Config, error) {
	root, err := config.FindRoot()
	if err != nil {
		return "", nil, err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return "", nil, err
	}

	return root, cfg, nil
}

// getPassphrase reads the passphrase from STORE_PASSPHRASE env var or prompts interactively.
func getPassphrase() (string, error) {
	if passphrase := os.Getenv("STORE_PASSPHRASE"); passphrase != "" {
		return passphrase, nil
	}

	return promptHiddenValue("Enter passphrase: ", "failed to read passphrase")
}

func promptHiddenValue(prompt, failureMessage string) (string, error) {
	fmt.Print(prompt)
	value, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("%s: %w", failureMessage, err)
	}
	return string(value), nil
}

func completeStoreNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	_ = cmd
	_ = toComplete

	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	_, cfg, err := findRootAndConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(cfg.Stores))
	for name := range cfg.Stores {
		names = append(names, name)
	}
	sort.Strings(names)

	return names, cobra.ShellCompDirectiveNoFileComp
}

// loadSecretsIfNeeded checks if any of the given store directories contain template
// placeholders. If so, prompts for passphrase and returns decrypted secrets.
// Returns nil if no rendering is needed.
func loadSecretsIfNeeded(root string, names ...string) (map[string]string, error) {
	for _, name := range names {
		storeDir := filepath.Join(root, name)
		needsRendering, err := render.NeedsRendering(storeDir)
		if err != nil {
			continue
		}
		if !needsRendering {
			continue
		}

		passphrase, err := getPassphrase()
		if err != nil {
			return nil, err
		}
		return secrets.Load(root, passphrase)
	}

	return nil, nil
}

// resolveTargetPath normalizes a target path for storage: expands ~ prefix,
// resolves relative paths to absolute. Tilde-prefixed paths are kept as-is
// for portability across machines.
func resolveTargetPath(target string) (string, error) {
	expanded, err := config.ExpandHome(target)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		target, err = filepath.Abs(target)
		if err != nil {
			return "", fmt.Errorf("failed to resolve target path: %w", err)
		}
	}
	return target, nil
}

// expandTargetPath fully expands a target path to an absolute path for comparison.
func expandTargetPath(target string) (string, error) {
	expanded, err := config.ExpandHome(target)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		expanded, err = filepath.Abs(expanded)
		if err != nil {
			return "", fmt.Errorf("failed to resolve target path: %w", err)
		}
	}
	return expanded, nil
}

// filterStoresByPlatform returns only stores whose When clause matches the current platform.
// Stores without a When clause are always included.
func filterStoresByPlatform(stores map[string]config.StoreEntry, info platform.Info) map[string]config.StoreEntry {
	filtered := make(map[string]config.StoreEntry)
	for name, entry := range stores {
		if entry.When == nil || entry.When.Matches(info) {
			filtered[name] = entry
		}
	}
	return filtered
}

func selectStores(stores map[string]config.StoreEntry, names []string) map[string]config.StoreEntry {
	if len(names) == 0 {
		return stores
	}

	filtered := make(map[string]config.StoreEntry)
	for _, name := range names {
		entry, exists := stores[name]
		if !exists {
			fmt.Printf("  warning: store %q not found in config\n", name)
			continue
		}
		filtered[name] = entry
	}

	return filtered
}

func printPlatformSkippedStores(selectedStores, filteredStores map[string]config.StoreEntry) {
	skippedNames := make([]string, 0)
	for name := range selectedStores {
		if _, ok := filteredStores[name]; !ok {
			skippedNames = append(skippedNames, name)
		}
	}

	sort.Strings(skippedNames)
	for _, name := range skippedNames {
		fmt.Printf("  skipping %s (platform mismatch)\n", name)
	}
}

func printStoredTarget(name, target string, hasFileMode bool) {
	if hasFileMode {
		fmt.Printf("  %s -> %s (files)\n", name, target)
		return
	}
	fmt.Printf("  %s -> %s\n", name, target)
}

func pluralizeCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// promptYesNo prints a prompt and reads a y/N response from stdin. Default is no.
func promptYesNo(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
