package tui

import "testing"

func TestStateMapping(t *testing.T) {
	cases := []struct {
		state State
		glyph string
		label string
	}{
		{StateLinked, GlyphLinked, "linked"},
		{StatePartial, GlyphPartial, "partial"},
		{StateMissing, GlyphMissing, "missing"},
		{StateConflict, GlyphConflct, "conflict"},
		{StateBroken, GlyphBroken, "broken"},
		{StateDrift, GlyphDrift, "drift"},
		{StateSkipped, GlyphSkipped, "skipped"},
	}
	for _, c := range cases {
		if got := c.state.Glyph(); got != c.glyph {
			t.Errorf("%v glyph = %q, want %q", c.state, got, c.glyph)
		}
		if got := c.state.Label(); got != c.label {
			t.Errorf("%v label = %q, want %q", c.state, got, c.label)
		}
		if c.state.Color() == "" {
			t.Errorf("%v color empty", c.state)
		}
	}
}

func TestMixEndpoints(t *testing.T) {
	if got := Mix(ColorFg, ColorEmber, 0); got != ColorFg {
		t.Errorf("Mix(t=0) = %s, want Fg %s", got, ColorFg)
	}
	if got := Mix(ColorFg, ColorEmber, 1); got != ColorEmber {
		t.Errorf("Mix(t=1) = %s, want Ember %s", got, ColorEmber)
	}
	// Midpoint: should be between the two.
	mid := Mix(ColorFg, ColorEmber, 0.5)
	if mid == ColorFg || mid == ColorEmber {
		t.Errorf("Mix(t=0.5) = %s, should be an interpolated value", mid)
	}
}
