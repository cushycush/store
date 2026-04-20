package tui

import "testing"

func TestPaletteExactPrefixOutranksContains(t *testing.T) {
	p := NewPalette()
	p.query.SetValue("rem")
	p.rebuildMatches()
	if len(p.filtered) == 0 {
		t.Fatalf("no matches for 'rem'")
	}
	top := p.commands[p.filtered[0].idx].Name
	if top != "remove" && top != "remove --all" && top != "rename" {
		t.Errorf("top match for 'rem' should start with 'rem'; got %q", top)
	}
}

func TestPaletteSubsequenceMatch(t *testing.T) {
	p := NewPalette()
	p.query.SetValue("sl") // matches "secret list" as subsequence
	p.rebuildMatches()
	found := false
	for _, m := range p.filtered {
		if p.commands[m.idx].Name == "secret list" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("subsequence 'sl' should match 'secret list'")
	}
}

func TestPaletteEveryCommandHasIntent(t *testing.T) {
	for _, c := range PaletteCommands() {
		if c.Build == nil {
			t.Errorf("command %q has no Build function", c.Name)
			continue
		}
		i := c.Build("")
		if i.Kind == IntentNone {
			t.Errorf("command %q builds IntentNone", c.Name)
		}
	}
}

func TestPaletteEmptyQueryMatchesAll(t *testing.T) {
	p := NewPalette()
	p.query.SetValue("")
	p.rebuildMatches()
	if len(p.filtered) != len(p.commands) {
		t.Errorf("empty query should match every command (got %d, want %d)",
			len(p.filtered), len(p.commands))
	}
}
