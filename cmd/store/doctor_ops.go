package main

import (
	"fmt"
	"strings"

	"github.com/cushycush/store/internal/doctor"
	"github.com/cushycush/store/internal/linker"
	storeops "github.com/cushycush/store/internal/store"
	"github.com/cushycush/store/internal/ui"
	"github.com/spf13/cobra"
)

func runDoctor(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	root, cfg, err := findRootAndConfig()
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

	fmt.Println(ui.Bold("Checking store health..."))
	fmt.Println()
	fmt.Printf("  %s %s stores configured\n", ui.DoctorOK(), ui.Bold(fmt.Sprintf("%d", len(cfg.Stores))))
	if brokenSymlinks == 0 {
		fmt.Printf("  %s all symlinks healthy\n", ui.DoctorOK())
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
		return ui.DoctorError()
	case "warning":
		return ui.DoctorWarn()
	default:
		return ui.DoctorInfo()
	}
}

func formatDoctorSummary(total, errors, warnings, infos int) string {
	if total == 0 {
		return fmt.Sprintf("%s issues found", ui.Bold("0"))
	}

	parts := make([]string, 0, 3)
	if errors > 0 {
		parts = append(parts, styledCount(errors, "error", "errors"))
	}
	if warnings > 0 {
		parts = append(parts, styledCount(warnings, "warning", "warnings"))
	}
	if infos > 0 {
		parts = append(parts, styledCount(infos, "info", "infos"))
	}

	return fmt.Sprintf("%s found (%s)", styledCount(total, "issue", "issues"), strings.Join(parts, ", "))
}
