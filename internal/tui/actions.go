package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ActionID tags an entry in the scoped action menu.
type ActionID int

const (
	ActionNone ActionID = iota
	ActionApply      // reconcile this one store
	ActionUnlink     // unlink this one store
	ActionDiff       // preview this one store
	ActionModify     // modify (opens palette arg flow)
	ActionRename     // rename
	ActionTargetOps  // open target sub-menu
	ActionPath       // copy path to activity log
	ActionRemove     // remove (destructive, confirmed)
)

// Actions is the row-scoped menu that opens on `enter`.
type Actions struct {
	StoreName string
	items     []actionItem
	groups    []actionGroup
	cursor    int
	chosen    ActionID
	cancel    bool
}

type actionItem struct {
	id    ActionID
	key   string
	label string
	note  string
}

// actionGroup denotes a visual break between item clusters. Groups are
// rendered in order with a blank line between them, giving the menu
// rhythm without adding headings.
type actionGroup []actionItem

// NewActions returns the menu for a store. Items are organized into
// three groups: the day-to-day actions (apply/unlink/diff), the edits
// (modify/rename/target/path), and the destructive action (remove) on
// its own at the bottom.
func NewActions(name string) *Actions {
	groups := []actionGroup{
		{
			{ActionApply, "a", "apply", "reconcile this store"},
			{ActionUnlink, "u", "unlink", "remove symlinks, keep config"},
			{ActionDiff, "d", "diff", "preview what apply would do"},
		},
		{
			{ActionModify, "m", "modify", "replace files or target"},
			{ActionRename, "n", "rename", "rename the store"},
			{ActionTargetOps, "t", "target…", "edit individual targets"},
			{ActionPath, "p", "path", "print the repo path"},
		},
		{
			{ActionRemove, "R", "remove", "unlink + delete entry (confirmed)"},
		},
	}
	var flat []actionItem
	for _, g := range groups {
		flat = append(flat, g...)
	}
	return &Actions{
		StoreName: name,
		items:     flat,
		groups:    groups,
	}
}

// Update processes a key; returns true once the menu should close.
func (a *Actions) Update(msg tea.Msg) bool {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	switch k.String() {
	case "esc":
		a.cancel = true
		return true
	case "enter":
		a.chosen = a.items[a.cursor].id
		return true
	case "j", "down":
		if a.cursor < len(a.items)-1 {
			a.cursor++
		}
	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
		}
	default:
		for _, it := range a.items {
			if k.String() == it.key {
				a.chosen = it.id
				return true
			}
		}
	}
	return false
}

// Chosen reports the selected action (ActionNone if cancelled).
func (a *Actions) Chosen() ActionID { return a.chosen }

// Cancelled reports whether the user pressed esc.
func (a *Actions) Cancelled() bool { return a.cancel }

// View renders the menu body, with blank separators between groups for
// visual rhythm.
func (a *Actions) View() string {
	var lines []string
	lines = append(lines,
		StyleMuted.Render("actions for ")+StyleBold.Render(a.StoreName),
		"",
	)
	globalIdx := 0
	for gi, g := range a.groups {
		if gi > 0 {
			lines = append(lines, "")
		}
		for _, it := range g {
			marker := "  "
			nameStyle := StyleFg
			noteStyle := StyleDim
			if globalIdx == a.cursor {
				marker = StyleEmber.Render(GlyphCursor) + " "
				nameStyle = StyleSelected
				noteStyle = StyleMuted
			}
			key := StyleHintKey.Render("[" + it.key + "]")
			lines = append(lines, marker+key+"  "+nameStyle.Render(padName(it.label, 10))+"  "+noteStyle.Render(it.note))
			globalIdx++
		}
	}
	return strings.Join(lines, "\n")
}

// Footer returns the key hint line.
func (a *Actions) Footer() string {
	return StyleDim.Render("enter select · esc close · j/k move")
}
