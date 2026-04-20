package tui

import "github.com/charmbracelet/bubbles/key"

// Keymap is the single source of truth for bindings and help text.
type Keymap struct {
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Back     key.Binding
	Enter    key.Binding
	Space    key.Binding
	Filter   key.Binding
	Palette  key.Binding
	Help     key.Binding
	Activity key.Binding
	Refresh  key.Binding
	Quit     key.Binding

	ApplyAll key.Binding
	Diff     key.Binding
	Remove   key.Binding
}

// DefaultKeymap returns the canonical keymap.
//
// Single-letter bindings are reserved for the handful of actions that earn
// frequent use. Everything else lives behind `:` (command palette) to keep
// the key surface vim-safe and below the memory cap.
func DefaultKeymap() Keymap {
	return Keymap{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Back:     key.NewBinding(key.WithKeys("esc", "h"), key.WithHelp("esc", "back")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "actions")),
		Space:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Palette:  key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Activity: key.NewBinding(key.WithKeys("\\"), key.WithHelp("\\", "log")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),

		ApplyAll: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "apply all")),
		Diff:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff")),
		Remove:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "remove")),
	}
}

// FooterHints returns the condensed hint strip shown at the bottom of the
// main view. Short on purpose; `?` opens the full help.
func (k Keymap) FooterHints() string {
	h := func(key, label string) string {
		return StyleHintKey.Render(key) + StyleHint.Render(" "+label)
	}
	sep := StyleDim.Render("   ")
	return h("j/k", "move") + sep +
		h("enter", "actions") + sep +
		h("space", "toggle") + sep +
		h("/", "filter") + sep +
		h(":", "command") + sep +
		h("?", "help")
}
