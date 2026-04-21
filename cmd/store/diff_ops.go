package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cushycush/store/v2/internal/linker"
	"github.com/cushycush/store/v2/internal/platform"
	storeops "github.com/cushycush/store/v2/internal/store"
	"github.com/cushycush/store/v2/internal/ui"
	"github.com/spf13/cobra"
)

func runDiff(_ *cobra.Command, _ []string, diffOnly []string) error {
	root, cfg, err := findRootAndConfig()
	if err != nil {
		return err
	}

	if len(cfg.Stores) == 0 {
		printNoStoresMessage()
		return nil
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
	File    string
	Target  string
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
		row := diffRow{Name: info.Name, File: info.File, Target: info.Target, Display: diffDisplay(info), Error: info.Error}

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
	labelWidth := len("[conflict]")
	for _, row := range rows {
		if len(row.Name) > nameWidth {
			nameWidth = len(row.Name)
		}
		if len(row.Display) > displayWidth {
			displayWidth = len(row.Display)
		}
	}

	for _, row := range rows {
		name := ui.StoreName(fmt.Sprintf("%-*s", nameWidth, row.Name))
		display := styledDiffDisplay(row, displayWidth)
		if row.Error != nil {
			// Error rows pad the label so the error message aligns.
			label := diffLabel(row.Label, labelWidth)
			fmt.Printf("  %s %s %s %s\n", name, display, label, ui.BoldRed(row.Error.Error()))
			continue
		}
		// Non-error rows have nothing after the label, so skip the padding
		// that would otherwise leave trailing whitespace.
		fmt.Printf("  %s %s %s\n", name, display, diffLabel(row.Label, 0))
	}
}

func formatDiffSummary(summary diffSummary) string {
	parts := []string{
		fmt.Sprintf("%s ok", ui.Bold(fmt.Sprintf("%d", summary.OK))),
		fmt.Sprintf("%s to create", ui.Bold(fmt.Sprintf("%d", summary.Create))),
		fmt.Sprintf("%s %s", ui.Bold(fmt.Sprintf("%d", summary.Conflict)), pluralWord(summary.Conflict, "conflict", "conflicts")),
		fmt.Sprintf("%s to replace", ui.Bold(fmt.Sprintf("%d", summary.Replace))),
	}
	if summary.Error > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", ui.Bold(fmt.Sprintf("%d", summary.Error)), pluralWord(summary.Error, "error", "errors")))
	}
	return fmt.Sprintf("Summary: %s", strings.Join(parts, ", "))
}

func styledDiffDisplay(row diffRow, width int) string {
	if row.File == "" {
		return ui.TargetPath(fmt.Sprintf("%-*s", width, row.Target))
	}

	display := fmt.Sprintf("%s %s %s", row.File, ui.Arrow(), ui.TargetPath(row.Target))
	padding := width - len(row.Display)
	if padding < 0 {
		padding = 0
	}
	return display + strings.Repeat(" ", padding)
}

func diffLabel(label string, width int) string {
	pad := func(marker, bracketed string) string {
		n := width - len(bracketed)
		if n < 0 {
			n = 0
		}
		return marker + strings.Repeat(" ", n)
	}
	switch label {
	case "ok":
		return pad(ui.DiffOK(), "[ok]")
	case "create":
		return pad(ui.DiffCreate(), "[create]")
	case "conflict":
		return pad(ui.DiffConflict(), "[conflict]")
	case "replace":
		return pad(ui.DiffReplace(), "[replace]")
	default:
		return pad(ui.DiffError(), "[error]")
	}
}

func pluralWord(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
