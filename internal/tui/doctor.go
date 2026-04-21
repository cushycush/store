package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cushycush/store/v2/internal/doctor"
)

// Doctor is the diagnostics overlay. Re-running is a single `r` keystroke.
type Doctor struct {
	root   string
	issues []doctor.Issue
	done   bool
}

// NewDoctor runs Check once and returns the overlay.
func NewDoctor(root string) *Doctor {
	return &Doctor{root: root, issues: doctor.Check(root)}
}

// Update handles keys.
func (o *Doctor) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch k.String() {
	case "esc", "q":
		o.done = true
	case "r":
		o.issues = doctor.Check(o.root)
	}
	return nil
}

// Done reports whether the overlay should close.
func (o *Doctor) Done() bool { return o.done }

// View renders the issue list, grouped by severity.
func (o *Doctor) View() string {
	var b strings.Builder
	if len(o.issues) == 0 {
		b.WriteString(StyleMuted.Render("no issues — store is healthy"))
		return b.String()
	}
	errs, warns, infos := splitBySeverity(o.issues)
	if len(errs) > 0 {
		b.WriteString(StyleMuted.Render("errors") + "\n")
		for _, i := range errs {
			b.WriteString("  " + renderIssue(i) + "\n")
		}
		b.WriteString("\n")
	}
	if len(warns) > 0 {
		b.WriteString(StyleMuted.Render("warnings") + "\n")
		for _, i := range warns {
			b.WriteString("  " + renderIssue(i) + "\n")
		}
		b.WriteString("\n")
	}
	if len(infos) > 0 {
		b.WriteString(StyleMuted.Render("info") + "\n")
		for _, i := range infos {
			b.WriteString("  " + renderIssue(i) + "\n")
		}
	}
	return b.String()
}

// Footer returns the key hint.
func (o *Doctor) Footer() string {
	return StyleDim.Render("r re-run · esc close")
}

func splitBySeverity(issues []doctor.Issue) (errs, warns, infos []doctor.Issue) {
	for _, i := range issues {
		switch i.Level {
		case "error":
			errs = append(errs, i)
		case "warn", "warning":
			warns = append(warns, i)
		default:
			infos = append(infos, i)
		}
	}
	return
}

func renderIssue(i doctor.Issue) string {
	var glyph string
	var color lipgloss.Color
	switch i.Level {
	case "error":
		glyph = "✕"
		color = ColorError
	case "warn", "warning":
		glyph = "⚠"
		color = ColorPartial
	default:
		glyph = "·"
		color = ColorMuted
	}
	return lipgloss.NewStyle().Foreground(color).Render(glyph) + "  " + StyleFg.Render(i.Message)
}
