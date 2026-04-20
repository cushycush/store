package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Help is the `?` overlay that lists every binding and the palette.
type Help struct {
	km   Keymap
	done bool
}

// NewHelp constructs the overlay for the given keymap.
func NewHelp(km Keymap) *Help { return &Help{km: km} }

// Update closes on any key.
func (o *Help) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(tea.KeyMsg); ok {
		o.done = true
	}
	return nil
}

// Done reports whether the overlay should close.
func (o *Help) Done() bool { return o.done }

// View renders the body, grouped by section.
func (o *Help) View() string {
	groups := []struct {
		title string
		rows  []row
	}{
		{"move", []row{
			{"j · k", "up and down"},
			{"g · G", "top and bottom"},
			{"esc · h", "back · close overlay"},
		}},
		{"stores", []row{
			{"enter", "actions for the selected store"},
			{"space", "link if missing, unlink if linked"},
			{"d", "diff — preview to the activity log"},
			{"A", "apply all (reconcile every store)"},
			{"R", "remove the selected store (confirmed)"},
			{"/", "filter"},
		}},
		{"commands", []row{
			{":", "open the command palette (all 20+ commands)"},
			{"\\", "fullscreen activity log"},
			{"r", "refresh"},
			{"?", "this help"},
			{"q", "quit"},
		}},
	}
	var b strings.Builder
	for i, g := range groups {
		b.WriteString(StyleMuted.Render(g.title))
		b.WriteString("\n")
		for _, r := range g.rows {
			b.WriteString("  " + StyleHintKey.Render(padName(r.key, 10)) + "  " + StyleFg.Render(r.label) + "\n")
		}
		if i < len(groups)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Footer returns the key hint.
func (o *Help) Footer() string {
	return StyleDim.Render("press any key to close")
}

type row struct {
	key   string
	label string
}
