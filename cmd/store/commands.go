package main

import (
	"fmt"
	"os"

	"github.com/cushycush/store/internal/ui"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "store",
		Short:   "A simpler alternative to GNU stow",
		Long:    "store manages symlinks for your dotfiles without requiring mirrored directory structures.",
		Version: version,
		RunE:    runStoreAll,
	}

	rootCmd.PersistentFlags().BoolVar(&forceBackups, "force", false, "create .bak backups without prompting")
	rootCmd.Flags().StringArrayVar(&onlyStores, "only", nil, "only store specific entries by name (repeatable)")

	rootCmd.AddCommand(
		newInitCmd(),
		newImportCmd(),
		newAddCmd(),
		newModifyCmd(),
		newRemoveCmd(),
		newRemoveAllCmd(),
		newStatusCmd(),
		newDiffCmd(),
		newDoctorCmd(),
		newVersionCmd(),
		newTargetCmd(),
		newSecretCmd(),
		newCompletionCmd(rootCmd),
	)

	return rootCmd
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

func newAddCmd() *cobra.Command {
	var target string
	var files []string
	var patterns []string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a store to config and create its symlinks",
		Long: `Adds a new store entry to config. Use flags to set the target path,
explicit files, and/or glob patterns for file-level symlinks.

Without --target, the entry is saved to config but no symlinks are created.
Without --files or --patterns, the entire directory is symlinked to the target.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(args[0], target, files, patterns)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path for the symlink")
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

	cmd := &cobra.Command{
		Use:   "modify <name>",
		Short: "Modify an existing store entry",
		Long: `Updates fields on an existing store entry. Each provided flag replaces
the entire field. Use --clear-files or --clear-patterns to remove those fields.

The old symlinks are removed before applying changes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModify(cmd, args[0], target, files, patterns, clearFiles, clearPatterns)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "new target path")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "replace file list (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "replace pattern list (repeatable)")
	cmd.Flags().BoolVar(&clearFiles, "clear-files", false, "remove all files from the entry")
	cmd.Flags().BoolVar(&clearPatterns, "clear-patterns", false, "remove all patterns from the entry")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a store's symlink",
		Long:  "Removes the symlink for the named store and deletes its config entry.",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}
	cmd.ValidArgsFunction = completeStoreNames
	return cmd
}

func newRemoveAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "removeall",
		Short: "Remove all store symlinks",
		Long:  "Removes symlinks and config entries for all stores defined in the config.",
		RunE:  runRemoveAll,
	}
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show symlink status",
		Long:  "Shows the symlink state for one or all stores.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runStatus,
	}
	cmd.ValidArgsFunction = completeStoreNames
	return cmd
}

func newDiffCmd() *cobra.Command {
	var diffOnly []string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Preview what store would change",
		Long:  "Shows what symlinks would be created, replaced, or conflict without making changes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, args, diffOnly)
		},
	}

	cmd.Flags().StringArrayVar(&diffOnly, "only", nil, "only diff specific entries by name (repeatable)")
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

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a target to a store",
		Long: `Adds a new target entry to an existing store. If the store currently uses
the single-target format, it is automatically migrated to the multi-target format.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTargetAdd(args[0], target, files, patterns)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path for the symlink (required)")
	mustMarkFlagRequired(cmd, "target")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "explicit files to symlink (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "glob patterns to match files (repeatable, supports **)")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newTargetRemoveCmd() *cobra.Command {
	var target string

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a target from a store",
		Long:  "Removes a specific target (by path) from a store and unlinks its symlinks.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTargetRemove(args[0], target)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path to remove (required)")
	mustMarkFlagRequired(cmd, "target")
	cmd.ValidArgsFunction = completeStoreNames

	return cmd
}

func newTargetModifyCmd() *cobra.Command {
	var target string
	var files []string
	var patterns []string
	var clearFiles bool
	var clearPatterns bool

	cmd := &cobra.Command{
		Use:   "modify <name>",
		Short: "Modify a target within a store",
		Long: `Modifies the files or patterns for a specific target within a store.
The target is identified by its path (-t flag). Each provided flag replaces
the entire field. Use --clear-files or --clear-patterns to remove those fields.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTargetModify(cmd, args[0], target, files, patterns, clearFiles, clearPatterns)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target path to modify (required)")
	mustMarkFlagRequired(cmd, "target")
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, "replace file list (repeatable)")
	cmd.Flags().StringArrayVarP(&patterns, "patterns", "p", nil, "replace pattern list (repeatable)")
	cmd.Flags().BoolVar(&clearFiles, "clear-files", false, "remove all files from the target")
	cmd.Flags().BoolVar(&clearPatterns, "clear-patterns", false, "remove all patterns from the target")
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
			Use:   "rm <name>",
			Short: "Remove a secret",
			Args:  cobra.ExactArgs(1),
			RunE:  runSecretRemove,
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
