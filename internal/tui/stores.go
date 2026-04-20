package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/linker"
	storeops "github.com/cushycush/store/internal/store"
)

// Row is a single store row in the ledger.
type Row struct {
	Name    string
	Summary string // target path, or "N targets"
	State   State
	Fresh   float64 // 0..1 spark intensity for fresh-change flourish
}

// Stores holds the state of the ledger row list.
type Stores struct {
	all    []Row
	view   []Row
	filter string
	cursor int
}

// NewStores returns an empty list. Populate with Refresh.
func NewStores() *Stores {
	return &Stores{}
}

// Refresh rebuilds rows from a loaded config, computing aggregate status
// for every store. freshMarks stamps the moment each store's state last
// changed; the renderer uses that to draw the fresh-change spark.
func (s *Stores) Refresh(root string, cfg *config.Config, freshMarks map[string]time.Time, now time.Time) {
	prev := make(map[string]State, len(s.all))
	for _, r := range s.all {
		prev[r.Name] = r.State
	}

	var names []string
	if cfg != nil {
		names = make([]string, 0, len(cfg.Stores))
		for n := range cfg.Stores {
			names = append(names, n)
		}
		sort.Strings(names)
	}

	rows := make([]Row, 0, len(names))
	for _, name := range names {
		entry := cfg.Stores[name]
		results := storeops.GetStatus(root, name, entry)
		state := aggregate(results)

		targets := entry.ResolvedTargets()
		summary := ""
		switch {
		case len(targets) == 0:
			summary = StyleDim.Render("(no target)")
		case len(targets) == 1:
			summary = targets[0].Target
		default:
			summary = plural(len(targets), "target", "targets")
		}

		row := Row{Name: name, Summary: summary, State: state}
		if old, ok := prev[name]; ok && old != state {
			freshMarks[name] = now
		}
		if ts, ok := freshMarks[name]; ok {
			age := now.Sub(ts)
			row.Fresh = decay(age, 2*time.Second)
			if row.Fresh <= 0 {
				delete(freshMarks, name)
			}
		}
		rows = append(rows, row)
	}
	s.all = rows
	s.rebuild()
}

// Filter updates the live filter.
func (s *Stores) Filter(q string) {
	s.filter = q
	s.rebuild()
}

// FilterQuery returns the current filter.
func (s *Stores) FilterQuery() string { return s.filter }

// Count returns the filtered row count.
func (s *Stores) Count() int { return len(s.view) }

// TotalCount returns the unfiltered row count.
func (s *Stores) TotalCount() int { return len(s.all) }

// Cursor returns the current cursor index within the filtered view.
func (s *Stores) Cursor() int { return s.cursor }

// Selected returns the name of the store under the cursor, or "".
func (s *Stores) Selected() string {
	if s.cursor < 0 || s.cursor >= len(s.view) {
		return ""
	}
	return s.view[s.cursor].Name
}

// Up moves the cursor one row toward the top.
func (s *Stores) Up() {
	if s.cursor > 0 {
		s.cursor--
	}
}

// Down moves the cursor one row toward the bottom.
func (s *Stores) Down() {
	if s.cursor < len(s.view)-1 {
		s.cursor++
	}
}

// Top moves the cursor to the first row.
func (s *Stores) Top() { s.cursor = 0 }

// Bottom moves the cursor to the last row.
func (s *Stores) Bottom() {
	s.cursor = len(s.view) - 1
	if s.cursor < 0 {
		s.cursor = 0
	}
}

// Rows returns the visible rows.
func (s *Stores) Rows() []Row { return s.view }

// Summary returns "N linked  M missing  ..." for the rule above the list.
func (s *Stores) Summary() string {
	counts := map[State]int{}
	for _, r := range s.all {
		counts[r.State]++
	}
	var parts []string
	for _, st := range []State{StateLinked, StatePartial, StateMissing, StateConflict, StateBroken, StateSkipped} {
		if n := counts[st]; n > 0 {
			c := lipgloss.NewStyle().Foreground(st.Color())
			parts = append(parts,
				c.Render(st.Glyph())+" "+StyleMuted.Render(plural(n, st.Label(), st.Label())),
			)
		}
	}
	return strings.Join(parts, "   ")
}

// HeaderLine returns "N stores" or "N of M stores" when a filter is active.
func (s *Stores) HeaderLine() string {
	if s.filter == "" {
		return plural(len(s.all), "store", "stores")
	}
	return plural(len(s.view), "match", "matches") + " of " + plural(len(s.all), "store", "stores")
}

// RenderRow renders one row of the store ledger at the given content width.
// `selected` is true for the row under the cursor; `reveal` is the 0..1
// fade-in opacity used during the initial staggered reveal.
func RenderRow(r Row, width int, selected bool, reveal float64) string {
	// Spark slot (fresh-change flourish).
	spark := "  "
	if r.Fresh > 0 {
		c := Mix(ColorFaint, ColorEmber, r.Fresh)
		spark = lipgloss.NewStyle().Foreground(c).Render(GlyphSpark) + " "
	}

	// Cursor marker.
	marker := "  "
	nameStyle := StyleFg
	if selected {
		marker = StyleEmber.Render(GlyphCursor) + " "
		nameStyle = StyleSelected
	}

	// State badge (right).
	stateGlyph := lipgloss.NewStyle().Foreground(r.State.Color()).Render(r.State.Glyph())
	stateLabel := lipgloss.NewStyle().Foreground(r.State.Color()).Render(r.State.Label())
	rightCol := stateGlyph + "  " + stateLabel

	// Name + summary left side.
	leftPrefixWidth := 4 // spark(2) + marker(2)
	nameWidth := 12
	name := nameStyle.Render(padName(r.Name, nameWidth))
	summary := StyleDim.Render(Clip(r.Summary, max(10, width-leftPrefixWidth-nameWidth-3-lipgloss.Width(rightCol))))

	// Assemble with fill between summary and right column.
	line := spark + marker + name + " " + summary
	used := lipgloss.Width(line)
	rightW := lipgloss.Width(rightCol)
	gap := width - used - rightW
	if gap < 1 {
		gap = 1
	}
	out := line + strings.Repeat(" ", gap) + rightCol

	// Initial reveal: tint everything toward faint.
	if reveal < 1 {
		out = dim(out, reveal)
	}
	return out
}

func padName(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s + " "
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

func aggregate(results []storeops.StatusInfo) State {
	if len(results) == 0 {
		return StateSkipped
	}
	// If any error, treat as conflict-like state.
	anyErr := false
	anyConflict := false
	linked := 0
	missing := 0
	for _, r := range results {
		if r.Error != nil {
			anyErr = true
			continue
		}
		switch r.Status {
		case linker.StatusLinked:
			linked++
		case linker.StatusMissing:
			missing++
		case linker.StatusConflict:
			anyConflict = true
		case linker.StatusBroken, linker.StatusDrift:
			return StatePartial
		}
	}
	if anyConflict {
		return StateConflict
	}
	if anyErr && linked == 0 {
		return StateConflict
	}
	total := len(results)
	if linked == total {
		return StateLinked
	}
	if missing == total {
		return StateMissing
	}
	return StatePartial
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return "1 " + singular
	}
	return itoa(n) + " " + pluralForm
}

func itoa(n int) string {
	// Small int helper to avoid importing strconv in several files.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func (s *Stores) rebuild() {
	if s.filter == "" {
		s.view = append(s.view[:0], s.all...)
	} else {
		q := strings.ToLower(s.filter)
		s.view = s.view[:0]
		for _, r := range s.all {
			if strings.Contains(strings.ToLower(r.Name), q) {
				s.view = append(s.view, r)
			}
		}
	}
	if s.cursor >= len(s.view) {
		s.cursor = len(s.view) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
}

// dim crossfades a rendered line toward the faint color at intensity
// (1-alpha). alpha=1 returns the line unchanged; alpha=0 returns fully faint.
//
// Because lipgloss has already rendered ANSI codes, re-tinting an already-
// styled string is lossy: we just replace the whole line with a single faint
// rendering of its visible text. This is fine during a brief initial reveal
// where colors don't matter yet.
func dim(s string, alpha float64) string {
	if alpha >= 1 {
		return s
	}
	visible := stripANSI(s)
	c := Mix(ColorFaint, ColorFg, alpha)
	return lipgloss.NewStyle().Foreground(c).Render(visible)
}

// stripANSI removes ANSI escape sequences from s. Tiny state machine; only
// used during reveal where we already own the content.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	in := false
	for _, r := range s {
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		if r == 0x1b {
			in = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
