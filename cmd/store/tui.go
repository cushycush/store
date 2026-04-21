package main

import (
	"fmt"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/tui"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive store dashboard",
		Long: `Opens a keyboard-driven dashboard showing every configured store, its
link state, and the recent activity log. Every CLI verb is accessible from
the dashboard via its command palette (press :).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := config.FindRoot()
			if err != nil {
				// Uninitialized repos get the TUI anyway — the uninit view
				// prompts the user to press `i` (init) or `:` (palette).
				root = "."
			}
			var cfg *config.Config
			if config.Exists(root) {
				cfg, err = config.Load(root)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
			}
			return tui.New(root, version, cfg).Run()
		},
	}
}
