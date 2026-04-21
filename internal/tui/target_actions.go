package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cushycush/store/v2/internal/config"
)

// TargetAction tags a per-target action picked from the target submenu.
type TargetAction int

const (
	TActionNone TargetAction = iota
	TActionApply
	TActionUnlink
	TActionModify
	TActionRemove
)

// targetPhase separates the "pick a target" phase from the "pick an
// action for that target" phase.
type targetPhase int

const (
	phasePickTarget targetPhase = iota
	phasePickAction
)

// TargetActions is the submenu reached from the main action menu's
// `target…` entry. Replaces the old stub that opened the command palette
// pre-filled with "target ".
type TargetActions struct {
	StoreName string
	Targets   []config.TargetEntry

	phase  targetPhase
	cursor int
	picked int

	items []targetActionItem

	chosen   TargetAction
	pickedTE config.TargetEntry
	done     bool
	cancel   bool
}

// targetActionItem is the per-target action list entry. Local to this
// overlay so its id type differs from the store-level ActionID.
type targetActionItem struct {
	id    TargetAction
	key   string
	label string
	note  string
}

// NewTargetActions constructs the overlay for a store with the given
// resolved targets. If targets is empty the overlay emits a friendly
// "no targets" body and esc-to-close.
func NewTargetActions(name string, targets []config.TargetEntry) *TargetActions {
	return &TargetActions{
		StoreName: name,
		Targets:   targets,
		items: []targetActionItem{
			{TActionApply, "a", "apply", "link this target"},
			{TActionUnlink, "u", "unlink", "remove this target's symlinks, keep config"},
			{TActionModify, "m", "modify", "replace file list for this target"},
			{TActionRemove, "R", "remove", "delete this target from the store"},
		},
	}
}

// Update processes a key message. Returns true when the overlay should close.
func (o *TargetActions) Update(msg tea.Msg) bool {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	switch o.phase {
	case phasePickTarget:
		switch k.String() {
		case "esc":
			o.cancel = true
			return true
		case "enter":
			if len(o.Targets) == 0 {
				o.cancel = true
				return true
			}
			o.picked = o.cursor
			o.pickedTE = o.Targets[o.cursor]
			o.phase = phasePickAction
			o.cursor = 0
		case "j", "down":
			if o.cursor < len(o.Targets)-1 {
				o.cursor++
			}
		case "k", "up":
			if o.cursor > 0 {
				o.cursor--
			}
		}
	case phasePickAction:
		switch k.String() {
		case "esc":
			// Back to target list.
			o.phase = phasePickTarget
			o.cursor = o.picked
		case "enter":
			o.chosen = o.items[o.cursor].id
			o.done = true
			return true
		case "j", "down":
			if o.cursor < len(o.items)-1 {
				o.cursor++
			}
		case "k", "up":
			if o.cursor > 0 {
				o.cursor--
			}
		default:
			for _, it := range o.items {
				if k.String() == it.key {
					o.chosen = it.id
					o.done = true
					return true
				}
			}
		}
	}
	return false
}

// Done reports whether the overlay should close. An overlay with an
// action chosen returns Chosen()/PickedTarget(); a cancelled overlay
// returns Cancelled() = true.
func (o *TargetActions) Done() bool { return o.done || o.cancel }

// Cancelled reports whether the user pressed esc without picking.
func (o *TargetActions) Cancelled() bool { return o.cancel }

// Chosen returns the action selected (TActionNone if cancelled).
func (o *TargetActions) Chosen() TargetAction { return o.chosen }

// PickedTarget returns the target entry the action applies to.
func (o *TargetActions) PickedTarget() config.TargetEntry { return o.pickedTE }

// View renders the overlay body.
func (o *TargetActions) View() string {
	var b strings.Builder
	b.WriteString(StyleMuted.Render("targets for ") + StyleBold.Render(o.StoreName) + "\n\n")
	if len(o.Targets) == 0 {
		b.WriteString(StyleDim.Render("  (no targets configured yet)") + "\n")
		return b.String()
	}
	switch o.phase {
	case phasePickTarget:
		for i, t := range o.Targets {
			marker := "  "
			nameStyle := StyleFg
			if i == o.cursor {
				marker = StyleEmber.Render(GlyphCursor) + " "
				nameStyle = StyleSelected
			}
			mode := "whole directory"
			if t.HasFileMode() {
				mode = fmt.Sprintf("%d files / %d patterns", len(t.Files), len(t.Patterns))
			}
			b.WriteString(marker + nameStyle.Render(t.Target) + "  " + StyleDim.Render(mode) + "\n")
		}
	case phasePickAction:
		b.WriteString(StyleDim.Render("  target ") + StyleFg.Render(o.pickedTE.Target) + "\n\n")
		for i, it := range o.items {
			marker := "  "
			nameStyle := StyleFg
			noteStyle := StyleDim
			if i == o.cursor {
				marker = StyleEmber.Render(GlyphCursor) + " "
				nameStyle = StyleSelected
				noteStyle = StyleMuted
			}
			key := StyleHintKey.Render("[" + it.key + "]")
			b.WriteString(marker + key + "  " + nameStyle.Render(padName(it.label, 10)) + "  " + noteStyle.Render(it.note) + "\n")
		}
	}
	return b.String()
}

// Footer returns the key hint line.
func (o *TargetActions) Footer() string {
	switch o.phase {
	case phasePickTarget:
		return StyleDim.Render("enter select · j/k move · esc close")
	case phasePickAction:
		return StyleDim.Render("enter select · esc back · j/k move")
	}
	return ""
}
