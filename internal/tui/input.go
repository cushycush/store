package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Input is a generic single-line input overlay used for prompts like
// "target path", "new store name", etc.
type Input struct {
	Title    string
	Hint     string
	input    textinput.Model
	done     bool
	cancel   bool
}

// NewInput constructs an input overlay. If mask is true the input is
// rendered as a password.
func NewInput(title, hint, placeholder string, mask bool) *Input {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	if mask {
		ti.EchoMode = textinput.EchoPassword
	}
	ti.Focus()
	return &Input{Title: title, Hint: hint, input: ti}
}

// Update processes a key.
func (o *Input) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch k.String() {
	case "esc":
		o.cancel = true
		o.done = true
		return nil
	case "enter":
		o.done = true
		return nil
	}
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return cmd
}

// Done reports whether the overlay should close.
func (o *Input) Done() bool { return o.done }

// Cancelled reports whether the user pressed esc.
func (o *Input) Cancelled() bool { return o.cancel }

// Value returns the trimmed input value.
func (o *Input) Value() string { return strings.TrimSpace(o.input.Value()) }

// View renders the overlay body.
func (o *Input) View() string {
	var b strings.Builder
	b.WriteString(StyleBold.Render(o.Title))
	if o.Hint != "" {
		b.WriteString("\n")
		b.WriteString(StyleDim.Render(o.Hint))
	}
	b.WriteString("\n\n  ")
	b.WriteString(o.input.View())
	return b.String()
}

// Footer returns the key hint line.
func (o *Input) Footer() string {
	return StyleDim.Render("enter accept · esc cancel")
}
