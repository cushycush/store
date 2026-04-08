package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/platform"
	storeops "github.com/cushycush/store/internal/store"
	"github.com/spf13/cobra"
)

func runDiff(_ *cobra.Command, _ []string, diffOnly []string) error {
	root, cfg, err := findRootAndConfig()
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
