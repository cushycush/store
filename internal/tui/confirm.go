package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Confirm is a destructive-action confirmation overlay. The user must type
// the expected word (often the store name) to proceed.
type Confirm struct {
	title    string
	body     []string
	expected string
	input    textinput.Model
	done     bool
	ok       bool
}

// NewConfirm builds the overlay. body lines are rendered above the input.
func NewConfirm(title string, body []string, expected string) *Confirm {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = expected
	ti.CharLimit = 128
	ti.Focus()
	return &Confirm{title: title, body: body, expected: expected, input: ti}
}

// Update handles keys. Closes on enter (confirms only if input == expected)
// or esc (cancels).
func (c *Confirm) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch k.String() {
	case "esc":
		c.done = true
		return nil
	case "enter":
		if strings.TrimSpace(c.input.Value()) == c.expected {
			c.ok = true
		}
		c.done = true
		return nil
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

// Done reports whether the overlay should close.
func (c *Confirm) Done() bool { return c.done }

// Ok reports whether the user confirmed successfully.
func (c *Confirm) Ok() bool { return c.ok }

// View renders the overlay body.
func (c *Confirm) View() string {
	var b strings.Builder
	b.WriteString(StyleBold.Render(c.title))
	b.WriteString("\n\n")
	for _, line := range c.body {
		b.WriteString(StyleMuted.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(StyleDim.Render("type ") + StyleFg.Render(c.expected) + StyleDim.Render(" to confirm:"))
	b.WriteString("\n  ")
	b.WriteString(c.input.View())
	return b.String()
}

// Footer returns the key hint line.
func (c *Confirm) Footer() string {
	return StyleDim.Render("enter confirm · esc cancel")
}
