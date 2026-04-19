package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/platform"
	"github.com/cushycush/store/internal/render"
	"github.com/cushycush/store/internal/secrets"
	storeops "github.com/cushycush/store/internal/store"
	"github.com/cushycush/store/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// applyListOps returns list with add entries appended (deduplicated) and
// remove entries filtered out. Used by modify/target modify to implement
// --add-file / --remove-file and --add-pattern / --remove-pattern.
func applyListOps(list, add, remove []string) []string {
	seen := make(map[string]bool, len(list))
	result := make([]string, 0, len(list)+len(add))
	for _, v := range list {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	for _, v := range add {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	if len(remove) == 0 {
		return result
	}
	rem := make(map[string]bool, len(remove))
	for _, v := range remove {
		rem[v] = true
	}
	filtered := result[:0]
	for _, v := range result {
		if !rem[v] {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

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
	fmt.Print(ui.Bold(prompt))
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

// buildRenderContext checks if any of the given store directories contain template
// placeholders. If so, prompts for passphrase and loads secrets. Always populates
// platform template data and user-defined vars from config.
func buildRenderContext(root string, cfg *config.Config, names ...string) (*storeops.RenderContext, error) {
	var secretMap map[string]string

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
		secretMap, err = secrets.Load(root, passphrase)
		if err != nil {
			return nil, err
		}
		break
	}

	info := platform.Detect()
	data := &render.TemplateData{
		Hostname: info.Hostname,
		OS:       info.OS,
		Arch:     info.Arch,
		Distro:   info.Distro,
		Shell:    info.Shell,
		Vars:     cfg.Vars,
	}

	return &storeops.RenderContext{
		Secrets:      secretMap,
		TemplateData: data,
	}, nil
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
			fmt.Println(ui.Dim(fmt.Sprintf("  warning: store %q not found in config", name)))
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
		fmt.Println(ui.Dim(fmt.Sprintf("  skipping %s (platform mismatch)", name)))
	}
}

func printStoredTarget(name, target string, hasFileMode bool) {
	if hasFileMode {
		fmt.Printf("  %s %s %s %s\n", ui.StoreName(name), ui.Arrow(), ui.TargetPath(target), ui.Dim("(files)"))
		return
	}
	fmt.Printf("  %s %s %s\n", ui.StoreName(name), ui.Arrow(), ui.TargetPath(target))
}

func pluralizeCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func styledCount(n int, singular, plural string) string {
	label := plural
	if n == 1 {
		label = singular
	}
	return ui.Bold(strconv.Itoa(n)) + " " + label
}

// promptYesNo prints a prompt and reads a y/N response from stdin. Default is no.
func promptYesNo(prompt string) bool {
	fmt.Printf("%s %s ", ui.Bold(prompt), ui.Dim("[y/N]"))
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
