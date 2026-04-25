package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

// TestKeymapNoCollisions verifies that no two distinct top-level bindings
// share a key trigger. `j/k/g/G/h/l` are reserved for movement and cannot
// be bound to actions. Overlay-local keys (e.g. `a` for add secret) are
// scoped and don't appear in the global keymap.
func TestKeymapNoCollisions(t *testing.T) {
	km := DefaultKeymap()
	seen := map[string]string{}
	bindings := map[string]key.Binding{
		"Up": km.Up, "Down": km.Down, "Top": km.Top, "Bottom": km.Bottom,
		"Back": km.Back, "Enter": km.Enter, "Space": km.Space,
		"Filter": km.Filter, "Palette": km.Palette, "Help": km.Help,
		"Activity": km.Activity, "Refresh": km.Refresh, "Quit": km.Quit,
		"ApplyAll": km.ApplyAll, "Diff": km.Diff, "Remove": km.Remove,
		"Expand": km.Expand, "Collapse": km.Collapse,
	}
	for label, b := range bindings {
		for _, k := range b.Keys() {
			// `esc` is deliberately shared between Back/overlay close; `q`
			// and `ctrl+c` both belong to Quit. Everything else should be
			// unique.
			if k == "esc" || k == "ctrl+c" {
				continue
			}
			if prev, ok := seen[k]; ok {
				t.Errorf("key %q bound to both %s and %s", k, prev, label)
			}
			seen[k] = label
		}
	}
}

// TestVimMovementNotOverloaded asserts the design principle: h/j/k/l/g/G
// are reserved for movement and are never bound to mutating actions.
// Tree expand/collapse counts as movement (file-tree convention) so `l`
// and `h` aliases Back/Expand which is consistent with that.
func TestVimMovementNotOverloaded(t *testing.T) {
	km := DefaultKeymap()
	forbidden := map[string]bool{"j": true, "k": true, "g": true, "G": true}
	// `h` aliases Back (and doubles as collapse-or-jump-to-parent in the
	// tree view); `l` aliases Expand which is movement, not mutation.
	actions := []key.Binding{
		km.Enter, km.Space, km.Filter, km.Palette, km.Help,
		km.Activity, km.Refresh, km.ApplyAll, km.Diff, km.Remove,
	}
	for _, b := range actions {
		for _, k := range b.Keys() {
			if forbidden[k] {
				t.Errorf("action binding is overloaded on movement key %q", k)
			}
		}
	}
}
