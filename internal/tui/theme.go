package tui

import (
	"fmt"
	"math"

	"github.com/charmbracelet/lipgloss"
)

// Palette. Warm near-monochrome tinted toward the ember accent.
// One signature color (Ember) carries the whole interface; severity hues
// stay dusty so they never compete with the accent.
//
// Background is assumed to be a user's terminal (typically dark). All
// foreground colors are tested for >= 4.5:1 contrast on backgrounds in
// the #0c0c0c..#202020 range.
var (
	ColorFg     = lipgloss.Color("#EDE6DC") // primary text, warm off-white
	ColorMuted  = lipgloss.Color("#A69B8A") // secondary text, warm gray
	ColorDim    = lipgloss.Color("#6B6558") // tertiary text, rules, hints
	ColorFaint  = lipgloss.Color("#3F3B35") // near-invisible tint for crossfades

	ColorEmber    = lipgloss.Color("#E89A3A") // signature accent
	ColorEmberLow = lipgloss.Color("#7A5324") // ember at low intensity (cursor trail tail)

	ColorLinked  = lipgloss.Color("#8AA27A") // dusty sage
	ColorPartial = lipgloss.Color("#D9A55E") // dim amber (close to accent family)
	ColorMissing = lipgloss.Color("#847C6E") // warm gray (missing is not an error)
	ColorError   = lipgloss.Color("#C27B6B") // terracotta
)

// Reusable styles. Kept minimal; composition happens at call sites.
var (
	StyleFg    = lipgloss.NewStyle().Foreground(ColorFg)
	StyleMuted = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleDim   = lipgloss.NewStyle().Foreground(ColorDim)
	StyleFaint = lipgloss.NewStyle().Foreground(ColorFaint)

	StyleEmber    = lipgloss.NewStyle().Foreground(ColorEmber)
	StyleEmberDim = lipgloss.NewStyle().Foreground(ColorEmberLow)

	StyleBold     = lipgloss.NewStyle().Foreground(ColorFg).Bold(true)
	StyleCursor   = lipgloss.NewStyle().Foreground(ColorEmber)
	StyleSelected = lipgloss.NewStyle().Foreground(ColorFg).Bold(true)
	StyleHint     = lipgloss.NewStyle().Foreground(ColorDim)
	StyleHintKey  = lipgloss.NewStyle().Foreground(ColorMuted)
)

// State glyphs. Single runes; never padded.
const (
	GlyphLinked  = "●"
	GlyphPartial = "◐"
	GlyphMissing = "○"
	GlyphConflct = "✕"
	GlyphBroken  = "⚡"
	GlyphDrift   = "◌"
	GlyphSkipped = "—"
	GlyphSpark   = "∗" // fresh-change flourish
	GlyphCursor  = "▸"
	GlyphHeart   = "·" // header heartbeat
)

// State is the aggregate state of a store or the per-file/per-target state
// used by the detail pane.
type State int

const (
	StateLinked State = iota
	StatePartial
	StateMissing
	StateConflict
	StateBroken
	StateDrift
	StateSkipped
)

// Glyph returns the single-rune glyph for a state.
func (s State) Glyph() string {
	switch s {
	case StateLinked:
		return GlyphLinked
	case StatePartial:
		return GlyphPartial
	case StateMissing:
		return GlyphMissing
	case StateConflict:
		return GlyphConflct
	case StateBroken:
		return GlyphBroken
	case StateDrift:
		return GlyphDrift
	case StateSkipped:
		return GlyphSkipped
	}
	return " "
}

// Color returns the color used for a state.
func (s State) Color() lipgloss.Color {
	switch s {
	case StateLinked:
		return ColorLinked
	case StatePartial, StateBroken, StateDrift:
		return ColorPartial
	case StateMissing:
		return ColorMissing
	case StateConflict:
		return ColorError
	case StateSkipped:
		return ColorDim
	}
	return ColorMuted
}

// Label returns the short word for a state.
func (s State) Label() string {
	switch s {
	case StateLinked:
		return "linked"
	case StatePartial:
		return "partial"
	case StateMissing:
		return "missing"
	case StateConflict:
		return "conflict"
	case StateBroken:
		return "broken"
	case StateDrift:
		return "drift"
	case StateSkipped:
		return "skipped"
	}
	return ""
}

// Mix linearly interpolates two hex colors. t=0 returns a, t=1 returns b.
// Used for fade-in animations and the cursor trail.
func Mix(a, b lipgloss.Color, t float64) lipgloss.Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	ar, ag, ab := hexToRGB(string(a))
	br, bg, bb := hexToRGB(string(b))
	r := int(math.Round(float64(ar) + (float64(br)-float64(ar))*t))
	g := int(math.Round(float64(ag) + (float64(bg)-float64(ag))*t))
	bl := int(math.Round(float64(ab) + (float64(bb)-float64(ab))*t))
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, bl))
}

func hexToRGB(h string) (int, int, int) {
	if len(h) == 7 && h[0] == '#' {
		var r, g, b int
		fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b)
		return r, g, b
	}
	return 0, 0, 0
}
