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
	"github.com/cushycush/store/internal/doctor"
	"github.com/cushycush/store/internal/hooks"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/platform"
	"github.com/cushycush/store/internal/render"
	"github.com/cushycush/store/internal/secrets"
	storeops "github.com/cushycush/store/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	version      = "dev"
	forceBackups bool
	onlyStores   []string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "store",
		Short:   "A simpler alternative to GNU stow",
		Long:    "store manages symlinks for your dotfiles without requiring mirrored directory structures.",
		Version: version,
		RunE:    runStoreAll,
	}
	rootCmd.PersistentFlags().BoolVar(&forceBackups, "force", false, "create .bak backups without prompting")
	rootCmd.Flags().StringArrayVar(&onlyStores, "only", nil, "only store specific entries by name (repeatable)")

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

	var diffOnly []string

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Preview what store would change",
		Long:  "Shows what symlinks would be created, replaced, or conflict without making changes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, args, diffOnly)
		},
	}
	diffCmd.Flags().StringArrayVar(&diffOnly, "only", nil, "only diff specific entries by name (repeatable)")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("store version %s\n", version)
		},
	}

	// --- target command group ---
	targetCmd := &cobra.Command{
		Use:   "target",
		Short: "Manage individual targets within a store",
		Long:  "Add, remove, or modify individual targets within a multi-target store.",
	}

	var targetAddTarget string
	var targetAddFiles []string
	var targetAddPatterns []string

	targetAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a target to a store",
		Long: `Adds a new target entry to an existing store. If the store currently uses
the single-target format, it is automatically migrated to the multi-target format.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTargetAdd(args[0], targetAddTarget, targetAddFiles, targetAddPatterns)
		},
	}
	targetAddCmd.Flags().StringVarP(&targetAddTarget, "target", "t", "", "target path for the symlink (required)")
	targetAddCmd.MarkFlagRequired("target")
	targetAddCmd.Flags().StringArrayVarP(&targetAddFiles, "files", "f", nil, "explicit files to symlink (repeatable)")
	targetAddCmd.Flags().StringArrayVarP(&targetAddPatterns, "patterns", "p", nil, "glob patterns to match files (repeatable, supports **)")

	var targetRemoveTarget string

	targetRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a target from a store",
		Long:  "Removes a specific target (by path) from a store and unlinks its symlinks.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTargetRemove(args[0], targetRemoveTarget)
		},
	}
	targetRemoveCmd.Flags().StringVarP(&targetRemoveTarget, "target", "t", "", "target path to remove (required)")
	targetRemoveCmd.MarkFlagRequired("target")

	var targetModTarget string
	var targetModFiles []string
	var targetModPatterns []string
	var targetModClearFiles bool
	var targetModClearPatterns bool

	targetModifyCmd := &cobra.Command{
		Use:   "modify <name>",
		Short: "Modify a target within a store",
		Long: `Modifies the files or patterns for a specific target within a store.
The target is identified by its path (-t flag). Each provided flag replaces
the entire field. Use --clear-files or --clear-patterns to remove those fields.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTargetModify(cmd, args[0], targetModTarget, targetModFiles, targetModPatterns, targetModClearFiles, targetModClearPatterns)
		},
	}
	targetModifyCmd.Flags().StringVarP(&targetModTarget, "target", "t", "", "target path to modify (required)")
	targetModifyCmd.MarkFlagRequired("target")
	targetModifyCmd.Flags().StringArrayVarP(&targetModFiles, "files", "f", nil, "replace file list (repeatable)")
	targetModifyCmd.Flags().StringArrayVarP(&targetModPatterns, "patterns", "p", nil, "replace pattern list (repeatable)")
	targetModifyCmd.Flags().BoolVar(&targetModClearFiles, "clear-files", false, "remove all files from the target")
	targetModifyCmd.Flags().BoolVar(&targetModClearPatterns, "clear-patterns", false, "remove all patterns from the target")

	targetCmd.AddCommand(targetAddCmd, targetRemoveCmd, targetModifyCmd)

	secretCmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage encrypted secrets",
		Long:  "Set, get, remove, and list secrets stored in .store/secrets.enc.",
	}

	secretSetCmd := &cobra.Command{
		Use:   "set <name> [value]",
		Short: "Set a secret value",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := config.FindRoot()
			if err != nil {
				return err
			}

			value := ""
			if len(args) == 2 {
				value = args[1]
			} else {
				fmt.Print("Enter secret value: ")
				pass, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if err != nil {
					return fmt.Errorf("failed to read secret value: %w", err)
				}
				value = string(pass)
			}

			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}

			secretMap, err := secrets.Load(root, passphrase)
			if err != nil {
				return err
			}
			secretMap[args[0]] = value

			if err := secrets.Save(root, passphrase, secretMap); err != nil {
				return err
			}

			fmt.Printf("Set secret %s\n", args[0])
			return nil
		},
	}

	secretGetCmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get a secret value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := config.FindRoot()
			if err != nil {
				return err
			}

			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}

			secretMap, err := secrets.Load(root, passphrase)
			if err != nil {
				return err
			}

			value, ok := secretMap[args[0]]
			if !ok {
				return fmt.Errorf("secret %q not found", args[0])
			}

			fmt.Println(value)
			return nil
		},
	}

	secretRmCmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := config.FindRoot()
			if err != nil {
				return err
			}

			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}

			secretMap, err := secrets.Load(root, passphrase)
			if err != nil {
				return err
			}

			if _, ok := secretMap[args[0]]; !ok {
				return fmt.Errorf("secret %q not found", args[0])
			}
			delete(secretMap, args[0])

			if err := secrets.Save(root, passphrase, secretMap); err != nil {
				return err
			}

			fmt.Printf("Removed secret %s\n", args[0])
			return nil
		},
	}

	secretListCmd := &cobra.Command{
		Use:   "list",
		Short: "List secret names",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := config.FindRoot()
			if err != nil {
				return err
			}

			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}

			secretMap, err := secrets.Load(root, passphrase)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(secretMap))
			for name := range secretMap {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				fmt.Println(name)
			}

			return nil
		},
	}

	secretCmd.AddCommand(secretSetCmd, secretGetCmd, secretRmCmd, secretListCmd)

	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate a completion script for your shell. Add the output to your
shell's startup file to enable tab completion.

  bash:       store completion bash > ~/.local/share/bash-completion/completions/store
  zsh:        store completion zsh > "${fpath[1]}/_store"
  fish:       store completion fish > ~/.config/fish/completions/store.fish
  powershell: store completion powershell | Out-String | Invoke-Expression`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	for _, cmd := range []*cobra.Command{modifyCmd, removeCmd, statusCmd, targetAddCmd, targetRemoveCmd, targetModifyCmd} {
		cmd.ValidArgsFunction = completeStoreNames
	}

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check store health",
		Long:  "Runs diagnostics on your store configuration and reports issues.",
		RunE:  runDoctor,
	}

	rootCmd.AddCommand(initCmd, addCmd, modifyCmd, removeCmd, removeAllCmd, statusCmd, diffCmd, versionCmd, targetCmd, secretCmd, completionCmd, doctorCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func completeStoreNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	_ = cmd
	_ = toComplete

	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	root, err := config.FindRoot()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(root)
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

func runDoctor(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	issues := doctor.Check(root)
	brokenSymlinks := 0
	for _, info := range storeops.GetStatusAll(root, cfg) {
		if info.Error == nil && info.Status == linker.StatusBroken {
			brokenSymlinks++
		}
	}

	errorCount, warningCount, infoCount := countDoctorIssues(issues)

	fmt.Println("Checking store health...")
	fmt.Println()
	fmt.Printf("  [ok] %d stores configured\n", len(cfg.Stores))
	if brokenSymlinks == 0 {
		fmt.Println("  [ok] all symlinks healthy")
	}

	if len(issues) > 0 {
		fmt.Println()
		for _, issue := range issues {
			fmt.Printf("  %s %s\n", doctorIndicator(issue.Level), issue.Message)
		}
	}

	fmt.Println()
	fmt.Println(formatDoctorSummary(len(issues), errorCount, warningCount, infoCount))

	if errorCount > 0 {
		return fmt.Errorf("doctor found %d error(s)", errorCount)
	}
	return nil
}

func countDoctorIssues(issues []doctor.Issue) (errors int, warnings int, infos int) {
	for _, issue := range issues {
		switch issue.Level {
		case "error":
			errors++
		case "warning":
			warnings++
		case "info":
			infos++
		}
	}
	return errors, warnings, infos
}

func doctorIndicator(level string) string {
	switch level {
	case "error":
		return "[error]"
	case "warning":
		return "[warn]"
	default:
		return "[info]"
	}
}

func formatDoctorSummary(total, errors, warnings, infos int) string {
	if total == 0 {
		return "0 issues found"
	}

	parts := make([]string, 0, 3)
	if errors > 0 {
		parts = append(parts, pluralizeCount(errors, "error", "errors"))
	}
	if warnings > 0 {
		parts = append(parts, pluralizeCount(warnings, "warning", "warnings"))
	}
	if infos > 0 {
		parts = append(parts, pluralizeCount(infos, "info", "infos"))
	}

	return fmt.Sprintf("%s found (%s)", pluralizeCount(total, "issue", "issues"), strings.Join(parts, ", "))
}

func runDiff(_ *cobra.Command, _ []string, diffOnly []string) error {
	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	cfg.Stores = selectStores(cfg.Stores, diffOnly)
	selectedStores := cfg.Stores
	filteredStores := filterStoresByPlatform(selectedStores, platform.Detect())
	printPlatformSkippedStores(selectedStores, filteredStores)
	cfg.Stores = filteredStores

	rows, summary := buildDiffReport(storeops.GetStatusAll(root, cfg))
	printDiffReport(rows)
	fmt.Println()
	fmt.Println(formatDiffSummary(summary))
	return nil
}

type diffRow struct {
	Name    string
	Display string
	Label   string
	Error   error
}

type diffSummary struct {
	OK       int
	Create   int
	Conflict int
	Replace  int
	Error    int
}

func buildDiffReport(results []storeops.StatusInfo) ([]diffRow, diffSummary) {
	rows := make([]diffRow, 0, len(results))
	summary := diffSummary{}

	for _, info := range results {
		row := diffRow{Name: info.Name, Display: diffDisplay(info), Error: info.Error}

		switch {
		case info.Error != nil:
			row.Label = "error"
			summary.Error++
		case info.Status == linker.StatusLinked:
			row.Label = "ok"
			summary.OK++
		case info.Status == linker.StatusMissing:
			row.Label = "create"
			summary.Create++
		case info.Status == linker.StatusBroken:
			row.Label = "replace"
			summary.Replace++
		case info.Status == linker.StatusConflict:
			row.Label = "conflict"
			summary.Conflict++
		default:
			row.Label = "error"
			row.Error = fmt.Errorf("unknown status: %v", info.Status)
			summary.Error++
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		if rows[i].Display != rows[j].Display {
			return rows[i].Display < rows[j].Display
		}
		return rows[i].Label < rows[j].Label
	})

	return rows, summary
}

func diffDisplay(info storeops.StatusInfo) string {
	if info.File != "" {
		return fmt.Sprintf("%s → %s", info.File, info.Target)
	}
	return info.Target
}

func printDiffReport(rows []diffRow) {
	nameWidth := len("store")
	displayWidth := len("path")
	for _, row := range rows {
		if len(row.Name) > nameWidth {
			nameWidth = len(row.Name)
		}
		if len(row.Display) > displayWidth {
			displayWidth = len(row.Display)
		}
	}

	for _, row := range rows {
		if row.Error != nil {
			fmt.Printf("  %-*s %-*s [%-8s] %v\n", nameWidth, row.Name, displayWidth, row.Display, row.Label, row.Error)
			continue
		}
		fmt.Printf("  %-*s %-*s [%-8s]\n", nameWidth, row.Name, displayWidth, row.Display, row.Label)
	}
}

func formatDiffSummary(summary diffSummary) string {
	parts := []string{
		fmt.Sprintf("%d ok", summary.OK),
		fmt.Sprintf("%d to create", summary.Create),
		pluralizeCount(summary.Conflict, "conflict", "conflicts"),
		fmt.Sprintf("%d to replace", summary.Replace),
	}
	if summary.Error > 0 {
		parts = append(parts, pluralizeCount(summary.Error, "error", "errors"))
	}
	return fmt.Sprintf("Summary: %s", strings.Join(parts, ", "))
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
		target, err = resolveTargetPath(target)
		if err != nil {
			return err
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
		secretMap, err := loadSecretsIfNeeded(root, name)
		if err != nil {
			return err
		}
		if err := storeWithConflictResolution(root, name, entry, secretMap); err != nil {
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

	if entry.IsMultiTarget() {
		return fmt.Errorf("store %q uses multiple targets; use 'store target modify' instead", name)
	}

	// Remove old symlinks before modifying.
	if err := storeops.StoreRemove(root, name, entry); err != nil {
		fmt.Printf("  warning: failed to remove old symlinks: %s\n", err)
	}

	// Apply modifications — each flag replaces the entire field.
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

	// Re-create symlinks with updated config.
	if entry.Target != "" {
		secretMap, err := loadSecretsIfNeeded(root, name)
		if err != nil {
			return err
		}
		if err := storeWithConflictResolution(root, name, entry, secretMap); err != nil {
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

	// Filter to only requested stores if --only was provided.
	if len(onlyStores) > 0 {
		filtered := make(map[string]config.StoreEntry)

		for _, name := range onlyStores {
			if entry, exists := cfg.Stores[name]; exists {
				filtered[name] = entry
			} else {
				fmt.Printf("  warning: store %q not found in config\n", name)
			}
		}

		cfg.Stores = filtered
	}

	selectedStores := cfg.Stores
	info := platform.Detect()
	filteredStores := filterStoresByPlatform(selectedStores, info)
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
	cfg.Stores = filteredStores
	if len(selectedStores) > 0 && len(cfg.Stores) == 0 {
		return nil
	}

	// Global pre hook.
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

	// Global post hook.
	if err := hooks.RunGlobal(root, "post-store", "link"); err != nil {
		fmt.Printf("  warning: %s\n", err)
	}

	return err
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

	targets := entry.ResolvedTargets()
	if len(targets) == 1 {
		fmt.Printf("Removed store %s (%s)\n", name, targets[0].Target)
	} else {
		fmt.Printf("Removed store %s (%d targets)\n", name, len(targets))
	}
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

	originalStores := cfg.Stores

	// Filter to only requested stores if --only was provided
	if len(onlyStores) > 0 {
		filtered := make(map[string]config.StoreEntry)

		for _, name := range onlyStores {
			if entry, exists := originalStores[name]; exists {
				filtered[name] = entry
			} else {
				fmt.Printf("  warning: store %q not found in config\n", name)
			}
		}

		originalStores = filtered
	}

	storesToRemove := filterStoresByPlatform(originalStores, platform.Detect())
	if len(originalStores) > 0 && len(storesToRemove) == 0 {
		return nil
	}

	// Global pre hook.
	if err := hooks.RunGlobal(root, "pre-remove", "unlink"); err != nil {
		return err
	}

	fmt.Println("Removing all stores:")
	var errors []error
	for name, entry := range storesToRemove {
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

	// Global post hook.
	if err := hooks.RunGlobal(root, "post-remove", "unlink"); err != nil {
		fmt.Printf("  warning: %s\n", err)
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
	cfg.Stores = filterStoresByPlatform(cfg.Stores, platform.Detect())

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

// getPassphrase reads the passphrase from STORE_PASSPHRASE env var or prompts interactively.
func getPassphrase() (string, error) {
	if p := os.Getenv("STORE_PASSPHRASE"); p != "" {
		return p, nil
	}
	fmt.Print("Enter passphrase: ")
	pass, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read passphrase: %w", err)
	}
	return string(pass), nil
}

// loadSecretsIfNeeded checks if any of the given store directories contain template
// placeholders. If so, prompts for passphrase and returns decrypted secrets.
// Returns nil if no rendering is needed.
func loadSecretsIfNeeded(root string, names ...string) (map[string]string, error) {
	needsSecrets := false
	for _, name := range names {
		storeDir := filepath.Join(root, name)
		needs, err := render.NeedsRendering(storeDir)
		if err != nil {
			continue
		}
		if needs {
			needsSecrets = true
			break
		}
	}
	if !needsSecrets {
		return nil, nil
	}

	passphrase, err := getPassphrase()
	if err != nil {
		return nil, err
	}
	return secrets.Load(root, passphrase)
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

func pluralizeCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func runTargetAdd(name, target string, files, patterns []string) error {
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
		return fmt.Errorf("store %q not found in config (use 'store add' to create it first)", name)
	}

	target, err = resolveTargetPath(target)
	if err != nil {
		return err
	}

	// Check for duplicate target path.
	expandedTarget, err := expandTargetPath(target)
	if err != nil {
		return err
	}
	for _, te := range entry.ResolvedTargets() {
		expandedExisting, err := expandTargetPath(te.Target)
		if err != nil {
			return err
		}
		if expandedExisting == expandedTarget {
			return fmt.Errorf("store %q already has a target %q", name, te.Target)
		}
	}

	// Migrate to multi-target format if currently single-target.
	entry.MigrateToMultiTarget()

	newTarget := config.TargetEntry{
		Target:   target,
		Files:    files,
		Patterns: patterns,
	}
	entry.Targets = append(entry.Targets, newTarget)

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	// Create symlinks for the new target.
	secretMap, err := loadSecretsIfNeeded(root, name)
	if err != nil {
		return err
	}
	if err := storeTargetWithConflictResolution(root, name, newTarget, secretMap); err != nil {
		return err
	}

	if newTarget.HasFileMode() {
		fmt.Printf("  %s -> %s (files)\n", name, target)
	} else {
		fmt.Printf("  %s -> %s\n", name, target)
	}
	return nil
}

func runTargetRemove(name, target string) error {
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

	expandedTarget, err := expandTargetPath(target)
	if err != nil {
		return err
	}

	// Migrate to multi-target so we can work with the Targets slice.
	entry.MigrateToMultiTarget()

	// Find and remove the target.
	found := -1
	for i, te := range entry.Targets {
		expandedExisting, err := expandTargetPath(te.Target)
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

	// Unlink the target being removed.
	if err := storeops.StoreRemoveTarget(root, name, entry.Targets[found]); err != nil {
		fmt.Printf("  warning: failed to remove symlinks: %s\n", err)
	}

	entry.Targets = append(entry.Targets[:found], entry.Targets[found+1:]...)

	if len(entry.Targets) == 0 {
		// No targets left — clear everything.
		entry.Target = ""
		entry.Files = nil
		entry.Patterns = nil
		entry.Targets = nil
	} else {
		// Migrate back to single-target if only one remains.
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

	expandedTarget, err := expandTargetPath(target)
	if err != nil {
		return err
	}

	// Migrate to multi-target so we can index into Targets.
	entry.MigrateToMultiTarget()

	found := -1
	for i, te := range entry.Targets {
		expandedExisting, err := expandTargetPath(te.Target)
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

	// Unlink old symlinks for this target.
	if err := storeops.StoreRemoveTarget(root, name, entry.Targets[found]); err != nil {
		fmt.Printf("  warning: failed to remove old symlinks: %s\n", err)
	}

	te := &entry.Targets[found]
	if cmd.Flags().Changed("files") {
		te.Files = files
	}
	if clearFiles {
		te.Files = nil
	}
	if cmd.Flags().Changed("patterns") {
		te.Patterns = patterns
	}
	if clearPatterns {
		te.Patterns = nil
	}

	// Migrate back to single-target if applicable.
	entry.MigrateToSingleTarget()

	cfg.Stores[name] = entry
	if err := config.Save(root, cfg); err != nil {
		return err
	}

	// Re-resolve from entry in case we migrated back.
	secretMap, err := loadSecretsIfNeeded(root, name)
	if err != nil {
		return err
	}
	for _, resolved := range entry.ResolvedTargets() {
		if resolved.Target == target {
			if err := storeTargetWithConflictResolution(root, name, resolved, secretMap); err != nil {
				return err
			}
			if resolved.HasFileMode() {
				fmt.Printf("  %s -> %s (files)\n", name, target)
			} else {
				fmt.Printf("  %s -> %s\n", name, target)
			}
			break
		}
	}
	return nil
}

// promptYesNo prints a prompt and reads a y/N response from stdin. Default is no.
func promptYesNo(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// printConflicts lists conflicts and what will happen to each file.
func printConflicts(conflicts []storeops.ConflictInfo) {
	fmt.Println("The following files conflict with store symlinks:")
	for _, c := range conflicts {
		kind := "file"
		if c.IsDir {
			kind = "directory"
		}
		fmt.Printf("  %s (%s -> will be moved to %s)\n", c.Target, kind, c.Source)
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
	for _, b := range backups {
		fmt.Printf(" %s\n", b)
	}
	fmt.Println()

	if !forceBackups {
		if !promptYesNo("Proceed with creating backups?") {
			return fmt.Errorf("aborted due to backup conflicts")
		}
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

	if len(conflicts) > 0 {
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
	}

	return storeops.StoreWithSecrets(root, name, entry, secrets)
}

// storeTargetWithConflictResolution checks for conflicts on a single target,
// prompts the user, resolves them, then creates symlinks.
func storeTargetWithConflictResolution(root, name string, te config.TargetEntry, secrets map[string]string) error {
	conflicts, err := storeops.CollectTargetConflicts(root, name, te)
	if err != nil {
		return err
	}

	if len(conflicts) > 0 {
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
	}

	return storeops.StoreTargetWithSecrets(root, name, te, secrets)
}

// storeAllWithConflictResolution checks for conflicts across all stores,
// prompts once, resolves, then creates all symlinks.
func storeAllWithConflictResolution(root string, cfg *config.Config, secrets map[string]string) error {
	if len(cfg.Stores) == 0 {
		return fmt.Errorf("no stores defined in config")
	}

	// Collect conflicts across all stores.
	var allConflicts []storeops.ConflictInfo
	for name, entry := range cfg.Stores {
		conflicts, err := storeops.CollectConflicts(root, name, entry)
		if err != nil {
			return err
		}
		allConflicts = append(allConflicts, conflicts...)
	}

	if len(allConflicts) > 0 {
		printConflicts(allConflicts)
		fmt.Println()
		if !promptYesNo("Move these files into the store and create symlinks?") {
			return fmt.Errorf("aborted due to unresolved conflicts")
		}
		if err := checkBackups(allConflicts); err != nil {
			return err
		}
		if err := storeops.ResolveConflicts(allConflicts); err != nil {
			return err
		}
		fmt.Println()
	}

	return storeops.StoreAllWithSecrets(root, cfg, secrets)
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
