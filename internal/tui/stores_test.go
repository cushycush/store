package tui

import "testing"

// seed populates s with synthetic store data, bypassing the real Refresh
// path which needs a config + filesystem.
func seed(s *Stores, items map[string]State) {
	s.data = map[string]storeData{}
	s.names = s.names[:0]
	for name, st := range items {
		s.data[name] = storeData{state: st, summary: name}
		s.names = append(s.names, name)
	}
	// keep names sorted to match Refresh's contract.
	for i := 0; i < len(s.names); i++ {
		for j := i + 1; j < len(s.names); j++ {
			if s.names[j] < s.names[i] {
				s.names[i], s.names[j] = s.names[j], s.names[i]
			}
		}
	}
	s.rebuild()
}

func TestStoresFilterAndCursor(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"nvim":   StateLinked,
		"shells": StatePartial,
		"git":    StateLinked,
		"tmux":   StateMissing,
	})

	if s.Count() != 4 {
		t.Fatalf("expected 4 rows, got %d", s.Count())
	}

	// Filter reduces the list; cursor clamps in-range.
	s.Bottom() // cursor = 3
	if s.Cursor() != 3 {
		t.Fatalf("Bottom() should leave cursor at 3, got %d", s.Cursor())
	}
	s.Filter("s")
	if s.Cursor() != 0 && s.Cursor() >= s.Count() {
		t.Errorf("cursor out of range after filter: cursor=%d count=%d", s.Cursor(), s.Count())
	}

	// Filter "n" matches "nvim" only.
	s.Filter("n")
	if s.Count() != 1 || s.Selected() != "nvim" {
		t.Errorf("filter 'n' should select nvim, got count=%d selected=%q", s.Count(), s.Selected())
	}

	// Clearing filter restores everything.
	s.Filter("")
	if s.Count() != 4 {
		t.Errorf("cleared filter should restore all rows; got %d", s.Count())
	}
}

func TestTreeCollapsedByDefault(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"desktop/hyprland": StateLinked,
		"desktop/waybar":   StateLinked,
		"editors/neovim":   StateLinked,
		"kmonad":           StateLinked,
	})
	// Expect 3 visible rows: two collapsed groups + 1 leaf store.
	if s.Count() != 3 {
		t.Fatalf("collapsed view count = %d, want 3", s.Count())
	}
	rows := s.Rows()
	if !rows[0].IsGroup || rows[0].Name != "desktop" {
		t.Errorf("row 0 = %+v, want desktop group", rows[0])
	}
	if !rows[1].IsGroup || rows[1].Name != "editors" {
		t.Errorf("row 1 = %+v, want editors group", rows[1])
	}
	if rows[2].IsGroup || rows[2].Name != "kmonad" {
		t.Errorf("row 2 = %+v, want kmonad leaf", rows[2])
	}
	if rows[0].DescendantCount != 2 {
		t.Errorf("desktop group count = %d, want 2", rows[0].DescendantCount)
	}
}

func TestTreeExpandShowsChildren(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"desktop/hyprland": StateLinked,
		"desktop/waybar":   StateLinked,
		"kmonad":           StateLinked,
	})
	// Cursor is on row 0 — desktop group.
	if !s.Expand() {
		t.Fatal("Expand() returned false on collapsed group")
	}
	// Now: desktop, hyprland, waybar, kmonad — 4 rows.
	if s.Count() != 4 {
		t.Fatalf("after expand, count = %d, want 4", s.Count())
	}
	rows := s.Rows()
	if rows[1].IsGroup || rows[1].Display != "hyprland" || rows[1].Depth != 1 {
		t.Errorf("row 1 = %+v, want depth-1 leaf hyprland", rows[1])
	}
	if rows[1].Name != "desktop/hyprland" {
		t.Errorf("row 1 Name = %q, want desktop/hyprland", rows[1].Name)
	}
}

func TestTreeCollapseFromChildJumpsToParent(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"desktop/hyprland": StateLinked,
		"desktop/waybar":   StateLinked,
	})
	s.Expand() // open desktop
	s.Down()   // move onto hyprland
	r, _ := s.SelectedRow()
	if r.IsGroup {
		t.Fatalf("expected leaf cursor, got %+v", r)
	}
	if !s.Collapse() {
		t.Fatal("Collapse() from child should jump to parent")
	}
	r, _ = s.SelectedRow()
	if !r.IsGroup || r.Name != "desktop" {
		t.Errorf("after Collapse, row = %+v, want desktop group", r)
	}
}

func TestTreeAggregatesGroupState(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"desktop/hyprland": StateLinked,
		"desktop/waybar":   StateMissing,
	})
	rows := s.Rows()
	if rows[0].State != StatePartial {
		t.Errorf("desktop group state = %v, want partial (linked + missing)", rows[0].State)
	}
}

func TestFilterFlattensTree(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"desktop/hyprland": StateLinked,
		"desktop/waybar":   StateLinked,
		"kmonad":           StateLinked,
	})
	s.Filter("way")
	if s.Count() != 1 {
		t.Fatalf("filter view count = %d, want 1", s.Count())
	}
	r, _ := s.SelectedRow()
	if r.IsGroup || r.Name != "desktop/waybar" {
		t.Errorf("filter row = %+v, want desktop/waybar leaf", r)
	}
	// Filter mode shows the full slash path, not just the leaf.
	if r.Display != "desktop/waybar" {
		t.Errorf("filter row Display = %q, want full path", r.Display)
	}
}

func TestSelectedStoreIgnoresGroups(t *testing.T) {
	s := NewStores()
	seed(s, map[string]State{
		"desktop/hyprland": StateLinked,
	})
	if got := s.SelectedStore(); got != "" {
		t.Errorf("cursor on group: SelectedStore = %q, want empty", got)
	}
	s.Expand()
	s.Down()
	if got := s.SelectedStore(); got != "desktop/hyprland" {
		t.Errorf("cursor on leaf: SelectedStore = %q, want desktop/hyprland", got)
	}
}

func TestAggregateStates(t *testing.T) {
	// Empty results => skipped
	if got := aggregate(nil); got != StateSkipped {
		t.Errorf("empty aggregate = %v, want skipped", got)
	}
}

func TestPadNameAtLeastFitsWidth(t *testing.T) {
	out := padName("nvim", 10)
	if len(out) < 10 {
		t.Errorf("padName should pad to at least 10 cols; got %q (len=%d)", out, len(out))
	}
}
