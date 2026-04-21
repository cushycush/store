package main

import (
	"fmt"
	"os"

	"github.com/cushycush/store/v2/internal/ui"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "store",
		Short:   "A simpler alternative to GNU stow",
		Long:    "store manages symlinks for your dotfiles without requiring mirrored directory structures.\n\nRun `store apply` to reconcile symlinks from .store/config.yaml.\nRun `store tui` for an interactive dashboard.",
		Version: version,
		// Suppress Cobra's reflex of dumping full flag help after every
		// error. Errors still print; usage dumps are available via --help.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.PersistentFlags().BoolVar(&forceBackups, "force", false, "create .bak backups without prompting")

	rootCmd.AddCommand(
		newApplyCmd(),
		newInitCmd(),
		newImportCmd(),
		newAdoptCmd(),
		newAddCmd(),
		newModifyCmd(),
		newRemoveCmd(),
		newRemoveAllCmd(),
		newListCmd(),
		newPathCmd(),
		newRenameCmd(),
		newEditCmd(),
		newStatusCmd(),
		newDiffCmd(),
		newDoctorCmd(),
		newVersionCmd(),
		newTargetCmd(),
		newSecretCmd(),
		newTUICmd(),
		newCompletionCmd(rootCmd),
	)

	return rootCmd
}

func newApplyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply all configured stores",
		Long: `Reconcile symlinks for all configured stores: create missing links,
replace broken ones, and report conflicts. Run this after cloning a dotfiles
repo on a new machine, or after changing .store/config.yaml.

Pass --dry-run to preview the plan without touching the filesystem.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return runDiff(cmd, args, onlyStores)
			}
			return runStoreAll(cmd, args)
		},
	}
	cmd.Flags().StringArrayVarP(&onlyStores, "only", "o", nil, "apply only the named stores (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without applying them (equivalent to store diff)")
	return cmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new store config",
		Long:  "Creates a .store/config.yaml file in the current directory.",
		RunE:  runInit,
	}
}

func newImportCmd() *cobra.Command {
	var scanDirs []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import existing symlinks into config",
		Long:  "Scans directories for symlinks pointing into the repo and imports them as store entries.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(scanDirs, dryRun)
		},
	}

	cmd.Flags().StringArrayVar(&scanDirs, "scan-dir", nil, "directories to scan for symlinks (default: ~, ~/.config, ~/.local/share, ~/.local/bin)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print discovered imports without writing config")

	return cmd
}

func newAdoptCmd() *cobra.Command {
	var name string
	var dryRun bool
	var files []string
	var patterns []string

	cmd := &cobra.Command{
		Use:   "adopt <path>",
		Short: "Adopt an existing file or directory into the store",
		Long: `Takes an existing file or directory, moves it into the repo, creates a
config entry, and symlinks back to the original location.

The store name is derived from the path basename (leading dots stripped).
Use --name to override the derived name.

Examples:
  store adopt ~/.config/nvim            # adopts whole directory as "nvim"
  store adopt ~/.zshrc                  # adopts single file as "zshrc"
  store adopt ~/.config/nvim --name vim # adopts as "vim" instead of "nvim"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdopt(args[0], name, dryRun, files, patterns)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "override the derived store name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would happen without making changes")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "only adopt specific files from a directory (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "only adopt files matching glob patterns (repeatable)")

	return cmd
}

func newAddCmd() *cobra.Command {
	var target string
	var files []string
	var patterns []string

	cmd := &cobra.Command{
		Use:   "add <name> [target]",
		Short: "Add a store to config and create its symlinks",
		Long: `Adds a new store entry to config. The target path may be passed
positionally or via -t/--target. Use flags to set explicit files or glob
patterns for file-level symlinks.

Without a target, the entry is saved to config but no symlinks are created.
Without --files or --patterns, the entire directory is symlinked to the target.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveOptionalPositionalTarget(cmd, args, target)
			if err != nil {
				return err
			}
			return runAdd(args[0], resolved, files, patterns)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path for the symlink (or pass positionally)")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "explicit files to symlink (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "glob patterns to match files (repeatable, supports **)")

	return cmd
}

func newModifyCmd() *cobra.Command {
	var target string
	var files []string
	var patterns []string
	var clearFiles bool
	var clearPatterns bool
	var ops modifyOps
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "modify <name>",
		Short: "Modify an existing store entry",
		Long: `Updates fields on an existing store entry.

--files and --patterns replace the entire list. --add-file, --remove-file,
--add-pattern, --remove-pattern apply incremental changes without touching
other entries. --clear-files and --clear-patterns empty the list entirely.
Flags compose in this order: clear, replace, add, remove.

The old symlinks are removed before applying changes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return previewModify(cmd, args[0], target, files, patterns, clearFiles, clearPatterns)
			}
			return runModify(cmd, args[0], target, files, patterns, clearFiles, clearPatterns, ops)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "new target path")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "replace file list (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "replace pattern list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.addFiles, "add-file", nil, "append a file to the file list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.removeFiles, "remove-file", nil, "remove a file from the file list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.addPatterns, "add-pattern", nil, "append a pattern to the pattern list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.removePatterns, "remove-pattern", nil, "remove a pattern from the pattern list (repeatable)")
	cmd.Flags().BoolVar(&clearFiles, "clear-files", false, "remove all files from the entry")
	cmd.Flags().BoolVar(&clearPatterns, "clear-patterns", false, "remove all patterns from the entry")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would change without applying it")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newRemoveCmd() *cobra.Command {
	var all bool
	var yes bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a store's symlink",
		Long: `Removes the symlink for the named store and deletes its config entry.

Use --all to remove every configured store at once. --all prompts for
confirmation unless --yes is passed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return fmt.Errorf("--all does not accept a store name")
				}
				if dryRun {
					return previewRemoveAll()
				}
				if !yes && !promptYesNo("Remove ALL configured stores?") {
					cmd.SilenceUsage = true
					return fmt.Errorf("aborted")
				}
				return runRemoveAll(cmd, args)
			}
			if len(args) == 0 {
				return fmt.Errorf("specify a store name or use --all")
			}
			if dryRun {
				return previewRemove(args[0])
			}
			return runRemove(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove every configured store")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt when using --all")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be removed without making changes")
	cmd.ValidArgsFunction = completeStoreNames
	return cmd
}

func newRemoveAllCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:        "removeall",
		Short:      "Remove all store symlinks",
		Long:       "Removes symlinks and config entries for all stores defined in the config.",
		Deprecated: "use `store remove --all` instead.",
		Hidden:     true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return previewRemoveAll()
			}
			return runRemoveAll(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be removed without making changes")
	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show symlink status",
		Long: `Shows the current symlink state (linked, missing, conflict, broken, drift)
for one or all stores.

For a one-line summary of configured stores without touching the filesystem,
use ` + "`store list`" + `. For a preview of the changes ` + "`store apply`" + ` would make,
use ` + "`store diff`" + `.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runStatus,
	}
	cmd.ValidArgsFunction = completeStoreNames
	return cmd
}

func newDiffCmd() *cobra.Command {
	var diffOnly []string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Preview what store would change",
		Long: `Shows which symlinks ` + "`store apply`" + ` would create, replace, conflict
with, or leave alone, without making any changes.

For the current state of existing symlinks (ignoring what apply would do),
use ` + "`store status`" + `. For a config-only view without touching the filesystem,
use ` + "`store list`" + `.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, args, diffOnly)
		},
	}

	cmd.Flags().StringArrayVarP(&diffOnly, "only", "o", nil, "only diff specific entries by name (repeatable)")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "doctor",
		Short:        "Check store health",
		Long:         "Runs diagnostics on your store configuration and reports issues.",
		SilenceUsage: true,
		RunE:         runDoctor,
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s version %s\n", ui.Bold("store"), ui.Bold(version))
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured stores",
		Long: `Prints a one-line summary of every store in the config, without touching
the filesystem. Useful for scripting or a quick glance at what's configured.

For live symlink state (linked, missing, conflict) use ` + "`store status`" + `.
For a preview of what ` + "`store apply`" + ` would change, use ` + "`store diff`" + `.`,
		Args: cobra.NoArgs,
		RunE: runList,
	}
}

func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path <name>",
		Short: "Print the on-disk path of a store directory",
		Long:  "Prints the absolute path to the store directory inside the repo. Useful for scripting: `cd $(store path nvim)`.",
		Args:  cobra.ExactArgs(1),
		RunE:  runPath,
	}
	cmd.ValidArgsFunction = completeStoreNames
	return cmd
}

func newRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a store",
		Long:  "Moves the store directory and updates the config entry. Re-links all targets under the new name.",
		Args:  cobra.ExactArgs(2),
		RunE:  runRename,
	}
	cmd.ValidArgsFunction = completeStoreNames
	return cmd
}

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open .store/config.yaml in $EDITOR",
		Long:  "Opens the config file in $EDITOR (falls back to vi). Does not validate on close — run `store doctor` afterwards.",
		Args:  cobra.NoArgs,
		RunE:  runEdit,
	}
}

func newTargetCmd() *cobra.Command {
	targetCmd := &cobra.Command{
		Use:   "target",
		Short: "Manage individual targets within a store",
		Long:  "Add, remove, or modify individual targets within a multi-target store.",
	}

	targetCmd.AddCommand(
		newTargetAddCmd(),
		newTargetRemoveCmd(),
		newTargetModifyCmd(),
	)

	return targetCmd
}

func newTargetAddCmd() *cobra.Command {
	var target string
	var files []string
	var patterns []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "add <name> [target]",
		Short: "Add a target to a store",
		Long: `Adds a new target entry to an existing store. If the store currently uses
the single-target format, it is automatically migrated to the multi-target format.

The target path may be passed positionally or with -t/--target.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvePositionalTarget(cmd, args, target)
			if err != nil {
				return err
			}
			if dryRun {
				return previewTargetAdd(args[0], resolved, files, patterns)
			}
			return runTargetAdd(args[0], resolved, files, patterns)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path for the symlink (or pass positionally)")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "explicit files to symlink (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "glob patterns to match files (repeatable, supports **)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be added without making changes")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newTargetRemoveCmd() *cobra.Command {
	var target string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "remove <name> [target]",
		Short: "Remove a target from a store",
		Long: `Removes a specific target (by path) from a store and unlinks its symlinks.

The target path may be passed positionally or with -t/--target.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvePositionalTarget(cmd, args, target)
			if err != nil {
				return err
			}
			if dryRun {
				return previewTargetRemove(args[0], resolved)
			}
			return runTargetRemove(args[0], resolved)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path to remove (or pass positionally)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be removed without making changes")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newTargetModifyCmd() *cobra.Command {
	var target string
	var files []string
	var patterns []string
	var clearFiles bool
	var clearPatterns bool
	var ops modifyOps
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "modify <name> [target]",
		Short: "Modify a target within a store",
		Long: `Modifies the files or patterns for a specific target within a store.
The target path may be passed positionally or with -t/--target.

--files and --patterns replace the entire list for that target. --add-file,
--remove-file, --add-pattern, --remove-pattern apply incremental changes.
--clear-files and --clear-patterns empty the list entirely. Flags compose
as clear → replace → add → remove.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolvePositionalTarget(cmd, args, target)
			if err != nil {
				return err
			}
			if dryRun {
				return previewTargetModify(cmd, args[0], resolved, files, patterns, clearFiles, clearPatterns)
			}
			return runTargetModify(cmd, args[0], resolved, files, patterns, clearFiles, clearPatterns, ops)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path to modify (or pass positionally)")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "replace file list (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "replace pattern list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.addFiles, "add-file", nil, "append a file to the file list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.removeFiles, "remove-file", nil, "remove a file from the file list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.addPatterns, "add-pattern", nil, "append a pattern to the pattern list (repeatable)")
	cmd.Flags().StringArrayVar(&ops.removePatterns, "remove-pattern", nil, "remove a pattern from the pattern list (repeatable)")
	cmd.Flags().BoolVar(&clearFiles, "clear-files", false, "remove all files from the target")
	cmd.Flags().BoolVar(&clearPatterns, "clear-patterns", false, "remove all patterns from the target")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would change without applying it")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newSecretCmd() *cobra.Command {
	secretCmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage encrypted secrets",
		Long:  "Set, get, remove, and list secrets stored in .store/secrets.enc.",
	}

	secretCmd.AddCommand(
		&cobra.Command{
			Use:   "set <name> [value]",
			Short: "Set a secret value",
			Args:  cobra.RangeArgs(1, 2),
			RunE:  runSecretSet,
		},
		&cobra.Command{
			Use:   "get <name>",
			Short: "Get a secret value",
			Args:  cobra.ExactArgs(1),
			RunE:  runSecretGet,
		},
		&cobra.Command{
			Use:     "remove <name>",
			Aliases: []string{"rm"},
			Short:   "Remove a secret",
			Args:    cobra.ExactArgs(1),
			RunE:    runSecretRemove,
		},
		&cobra.Command{
			Use:   "list",
			Short: "List secret names",
			Args:  cobra.NoArgs,
			RunE:  runSecretList,
		},
	)

	return secretCmd
}

func newCompletionCmd(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
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
}

func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}
