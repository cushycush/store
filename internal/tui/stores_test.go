package tui

import "testing"

func TestStoresFilterAndCursor(t *testing.T) {
	s := NewStores()
	// Inject rows by hand; Refresh needs a full config + filesystem.
	s.all = []Row{
		{Name: "nvim", State: StateLinked},
		{Name: "shells", State: StatePartial},
		{Name: "git", State: StateLinked},
		{Name: "tmux", State: StateMissing},
	}
	s.rebuild()

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
