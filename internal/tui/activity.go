package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ActivityKind tags a log entry.
type ActivityKind int

const (
	ActivityOK ActivityKind = iota
	ActivityWarn
	ActivityErr
)

// Entry is a single activity line.
type Entry struct {
	Kind    ActivityKind
	Message string
	At      time.Time
}

// Activity is the bounded log of recent operations.
type Activity struct {
	entries []Entry
	cap     int
	scroll  int // 0 = pinned to tail
	now     func() time.Time
}

// NewActivity returns an empty log with the given capacity.
func NewActivity(cap int) *Activity {
	if cap < 32 {
		cap = 32
	}
	return &Activity{cap: cap, now: time.Now}
}

// Append records a new entry. Drops oldest when over capacity.
func (a *Activity) Append(kind ActivityKind, msg string) {
	a.entries = append(a.entries, Entry{Kind: kind, Message: msg, At: a.now()})
	if len(a.entries) > a.cap {
		a.entries = a.entries[len(a.entries)-a.cap:]
	}
	a.scroll = 0
}

// Ok, Warn, Err are convenience wrappers around Append.
func (a *Activity) Ok(msg string)   { a.Append(ActivityOK, msg) }
func (a *Activity) Warn(msg string) { a.Append(ActivityWarn, msg) }
func (a *Activity) Err(msg string)  { a.Append(ActivityErr, msg) }

// Entries returns a copy of all buffered entries, oldest first.
func (a *Activity) Entries() []Entry {
	out := make([]Entry, len(a.entries))
	copy(out, a.entries)
	return out
}

// Last returns the most recent entry, or (Entry{}, false) if empty.
func (a *Activity) Last() (Entry, bool) {
	if len(a.entries) == 0 {
		return Entry{}, false
	}
	return a.entries[len(a.entries)-1], true
}

// Empty returns true when no entries have been recorded.
func (a *Activity) Empty() bool { return len(a.entries) == 0 }

// ScrollUp moves the view toward older entries.
func (a *Activity) ScrollUp() {
	if a.scroll < len(a.entries)-1 {
		a.scroll++
	}
}

// ScrollDown moves the view toward newer entries.
func (a *Activity) ScrollDown() {
	if a.scroll > 0 {
		a.scroll--
	}
}

// RenderLine returns the single-line summary shown in the main view, or ""
// if the log is empty.
func (a *Activity) RenderLine(width int) string {
	e, ok := a.Last()
	if !ok {
		return ""
	}
	glyph, color := activityGlyph(e.Kind)
	age := formatAge(a.now().Sub(e.At))
	return StyleMuted.Render("recent") + "   " +
		lipgloss.NewStyle().Foreground(color).Render(glyph) + "  " +
		StyleFg.Render(Clip(e.Message, max0(width-24, 20))) + "   " +
		StyleDim.Render(age)
}

// RenderFull returns the fullscreen log rendered to the given dimensions.
func (a *Activity) RenderFull(width, height int) string {
	if len(a.entries) == 0 {
		return StyleDim.Render("  (no activity yet)")
	}
	end := len(a.entries) - a.scroll
	start := end - height
	if start < 0 {
		start = 0
	}
	var lines []string
	for i := start; i < end; i++ {
		e := a.entries[i]
		glyph, color := activityGlyph(e.Kind)
		age := formatAge(a.now().Sub(e.At))
		lines = append(lines,
			"  "+StyleDim.Render(fmt.Sprintf("%-8s", age))+"  "+
				lipgloss.NewStyle().Foreground(color).Render(glyph)+"  "+
				StyleFg.Render(e.Message),
		)
	}
	return strings.Join(lines, "\n")
}

func activityGlyph(k ActivityKind) (string, lipgloss.Color) {
	switch k {
	case ActivityWarn:
		return "⚠", ColorPartial
	case ActivityErr:
		return "✕", ColorError
	default:
		return "✓", ColorLinked
	}
}

func formatAge(d time.Duration) string {
	s := int64(d.Seconds())
	if s < 0 {
		s = 0
	}
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm", s/60)
	default:
		return fmt.Sprintf("%dh", s/3600)
	}
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}
