package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/linker"
	storeops "github.com/cushycush/store/v2/internal/store"
)

// Row is a single visible line in the ledger. It is either a leaf store row
// or a group row aggregating descendants. Groups exist when store names
// share a slash-prefix (e.g. desktop/hyprland and desktop/waybar).
type Row struct {
	// Name is the full slash path. For groups, the group path ("desktop").
	// For stores, the full store key ("desktop/hyprland").
	Name string
	// Display is the rendered label: leaf segment when nested, full path
	// when at depth 0 or in flattened filter mode.
	Display string
	// Depth is the number of parent groups preceding this row.
	Depth int
	// IsGroup marks group rows so navigation/actions can branch.
	IsGroup bool
	// Expanded reports whether a group's children are currently visible.
	Expanded bool
	// DescendantCount is the number of leaf stores under a group.
	DescendantCount int
	// Summary is the dim right-side hint (target path or "N stores").
	Summary string
	// State is the aggregate (for groups) or per-store state.
	State State
	// Fresh is the spark intensity (0..1) for the fresh-change flourish.
	Fresh float64
}

// Stores holds the state of the ledger row list.
type Stores struct {
	// data holds per-store info keyed by full slash name. Recomputed by
	// Refresh from a loaded config + on-disk state.
	data map[string]storeData
	// names is the sorted list of full slash names from the last refresh.
	names []string
	// expanded maps group full-path → true. Missing keys are collapsed.
	// Persists across refreshes so the user's expansion state survives
	// config reloads.
	expanded map[string]bool

	view   []Row
	filter string
	cursor int
}

// storeData holds the per-store fields produced by Refresh that get
// projected into the visible Row.
type storeData struct {
	state   State
	summary string
	fresh   float64
}

// NewStores returns an empty list. Populate with Refresh.
func NewStores() *Stores {
	return &Stores{
		data:     map[string]storeData{},
		expanded: map[string]bool{},
	}
}

// Refresh rebuilds rows from a loaded config, computing aggregate status
// for every store. freshMarks stamps the moment each store's state last
// changed; the renderer uses that to draw the fresh-change spark.
func (s *Stores) Refresh(root string, cfg *config.Config, freshMarks map[string]time.Time, now time.Time) {
	prev := s.data
	s.data = make(map[string]storeData, len(prev))
	s.names = s.names[:0]

	if cfg != nil {
		for n := range cfg.Stores {
			s.names = append(s.names, n)
		}
		sort.Strings(s.names)
	}

	for _, name := range s.names {
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

		d := storeData{state: state, summary: summary}
		if old, ok := prev[name]; ok && old.state != state {
			freshMarks[name] = now
		}
		if ts, ok := freshMarks[name]; ok {
			age := now.Sub(ts)
			d.fresh = decay(age, 2*time.Second)
			if d.fresh <= 0 {
				delete(freshMarks, name)
			}
		}
		s.data[name] = d
	}

	// Drop expansion state for groups that no longer have any descendants.
	if len(s.expanded) > 0 {
		live := liveGroups(s.names)
		for g := range s.expanded {
			if !live[g] {
				delete(s.expanded, g)
			}
		}
	}

	s.rebuild()
}

// liveGroups returns the set of group paths that currently have at least
// one store underneath them.
func liveGroups(names []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		idx := 0
		for {
			i := strings.IndexByte(n[idx:], '/')
			if i < 0 {
				break
			}
			out[n[:idx+i]] = true
			idx += i + 1
		}
	}
	return out
}

// Filter updates the live filter.
func (s *Stores) Filter(q string) {
	s.filter = q
	s.rebuild()
}

// FilterQuery returns the current filter.
func (s *Stores) FilterQuery() string { return s.filter }

// Count returns the visible row count (groups + stores).
func (s *Stores) Count() int { return len(s.view) }

// TotalCount returns the unfiltered store count (excludes groups).
func (s *Stores) TotalCount() int { return len(s.names) }

// Cursor returns the current cursor index within the view.
func (s *Stores) Cursor() int { return s.cursor }

// Selected returns the full name of the row under the cursor (group path
// or store name), or "".
func (s *Stores) Selected() string {
	r, ok := s.SelectedRow()
	if !ok {
		return ""
	}
	return r.Name
}

// SelectedStore returns the full name only if the cursor sits on a leaf
// store row. Use this when an action only makes sense for stores.
func (s *Stores) SelectedStore() string {
	r, ok := s.SelectedRow()
	if !ok || r.IsGroup {
		return ""
	}
	return r.Name
}

// SelectedRow returns the row under the cursor.
func (s *Stores) SelectedRow() (Row, bool) {
	if s.cursor < 0 || s.cursor >= len(s.view) {
		return Row{}, false
	}
	return s.view[s.cursor], true
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

// Expand opens the group under the cursor. Returns true if anything
// changed (the cursor was on a collapsed group).
func (s *Stores) Expand() bool {
	r, ok := s.SelectedRow()
	if !ok || !r.IsGroup || r.Expanded {
		return false
	}
	s.expanded[r.Name] = true
	s.rebuild()
	return true
}

// Collapse shuts the group under the cursor (or, if the cursor is on a
// nested store row, jumps up to its parent group and collapses that).
// Returns true if anything changed.
func (s *Stores) Collapse() bool {
	r, ok := s.SelectedRow()
	if !ok {
		return false
	}
	if r.IsGroup && r.Expanded {
		delete(s.expanded, r.Name)
		s.rebuild()
		return true
	}
	if !r.IsGroup && r.Depth > 0 {
		// Walk up to the nearest visible parent group.
		parent := groupParent(r.Name)
		for parent != "" {
			if i := s.indexOfGroup(parent); i >= 0 {
				s.cursor = i
				return true
			}
			parent = groupParent(parent)
		}
	}
	return false
}

// ToggleExpand flips the expansion state of the group under the cursor.
// No-op for store rows.
func (s *Stores) ToggleExpand() bool {
	r, ok := s.SelectedRow()
	if !ok || !r.IsGroup {
		return false
	}
	if r.Expanded {
		delete(s.expanded, r.Name)
	} else {
		s.expanded[r.Name] = true
	}
	s.rebuild()
	return true
}

// ExpandAll opens every group. Used by the palette command and tests.
func (s *Stores) ExpandAll() {
	for _, g := range allGroups(s.names) {
		s.expanded[g] = true
	}
	s.rebuild()
}

// CollapseAll closes every group.
func (s *Stores) CollapseAll() {
	s.expanded = map[string]bool{}
	s.rebuild()
}

// Rows returns the visible rows.
func (s *Stores) Rows() []Row { return s.view }

// Window returns the slice of rows that should be visible given the
// available height, keeping the cursor in view. topElided is the count
// of rows clipped above the window; bottomElided is the count below.
// When both are zero the whole list fits.
func (s *Stores) Window(height int) (visible []Row, topElided, bottomElided int) {
	if height <= 0 || len(s.view) == 0 {
		return nil, 0, 0
	}
	if len(s.view) <= height {
		return s.view, 0, 0
	}
	start := s.cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(s.view) {
		end = len(s.view)
		start = end - height
	}
	return s.view[start:end], start, len(s.view) - end
}

// Summary returns "N linked  M missing  ..." for the rule above the list.
func (s *Stores) Summary() string {
	counts := map[State]int{}
	for _, name := range s.names {
		counts[s.data[name].state]++
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
	total := len(s.names)
	if s.filter == "" {
		return plural(total, "store", "stores")
	}
	matches := 0
	for _, r := range s.view {
		if !r.IsGroup {
			matches++
		}
	}
	return plural(matches, "match", "matches") + " of " + plural(total, "store", "stores")
}

// indexOfGroup returns the view index of the given group path, or -1.
func (s *Stores) indexOfGroup(path string) int {
	for i, r := range s.view {
		if r.IsGroup && r.Name == path {
			return i
		}
	}
	return -1
}

// groupParent returns the parent group path of a slash-named row, or "".
func groupParent(name string) string {
	idx := strings.LastIndex(name, "/")
	if idx < 0 {
		return ""
	}
	return name[:idx]
}

// allGroups returns every group path that exists across the given store
// names.
func allGroups(names []string) []string {
	live := liveGroups(names)
	out := make([]string, 0, len(live))
	for g := range live {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// RenderRow renders one row of the store ledger at the given content width.
// `selected` is true for the row under the cursor; `reveal` is the 0..1
// fade-in opacity used during the initial staggered reveal.
func RenderRow(r Row, width int, selected bool, reveal float64) string {
	// Spark slot (fresh-change flourish, leaf rows only).
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

	// Per-depth indent. Depth 0 means the row sits at the top level.
	indent := strings.Repeat("  ", r.Depth)

	// Group glyph: "+" collapsed, "−" expanded. Non-group rows leave the
	// slot blank so leaf names align with their group's title text.
	groupGlyph := "  "
	if r.IsGroup {
		if r.Expanded {
			groupGlyph = StyleDim.Render("−") + " "
		} else {
			groupGlyph = StyleEmber.Render("+") + " "
		}
	}

	// State badge (right).
	stateGlyph := lipgloss.NewStyle().Foreground(r.State.Color()).Render(r.State.Glyph())
	stateLabel := lipgloss.NewStyle().Foreground(r.State.Color()).Render(r.State.Label())
	rightCol := stateGlyph + "  " + stateLabel

	// Compose left side.
	leftPrefixWidth := 4 + lipgloss.Width(indent) + lipgloss.Width(groupGlyph)
	nameWidth := 12
	display := r.Display
	if display == "" {
		display = r.Name
	}
	name := nameStyle.Render(padName(display, nameWidth))
	summary := StyleDim.Render(Clip(r.Summary, max(10, width-leftPrefixWidth-nameWidth-3-lipgloss.Width(rightCol))))

	line := spark + marker + indent + groupGlyph + name + " " + summary
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

// aggregateStates rolls up a slice of leaf states into a single group
// state. Conflict/error wins; otherwise all-linked, all-missing, or
// partial.
func aggregateStates(states []State) State {
	if len(states) == 0 {
		return StateSkipped
	}
	for _, st := range states {
		if st == StateConflict || st == StateBroken {
			return StateConflict
		}
	}
	allLinked := true
	allMissing := true
	for _, st := range states {
		if st != StateLinked {
			allLinked = false
		}
		if st != StateMissing && st != StateSkipped {
			allMissing = false
		}
	}
	if allLinked {
		return StateLinked
	}
	if allMissing {
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
	prevKey := ""
	if r, ok := s.SelectedRow(); ok {
		prevKey = rowKey(r)
	}

	if s.filter != "" {
		s.view = s.flattenedFilteredRows()
	} else {
		s.view = s.treeRows()
	}

	if prevKey != "" {
		for i, r := range s.view {
			if rowKey(r) == prevKey {
				s.cursor = i
				return
			}
		}
		// If we collapsed the parent of a previously-selected store,
		// land the cursor on that parent group.
		if strings.HasPrefix(prevKey, "s:") {
			ancestor := groupParent(prevKey[len("s:"):])
			for ancestor != "" {
				if idx := s.indexOfGroup(ancestor); idx >= 0 {
					s.cursor = idx
					return
				}
				ancestor = groupParent(ancestor)
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

func rowKey(r Row) string {
	if r.IsGroup {
		return "g:" + r.Name
	}
	return "s:" + r.Name
}

// flattenedFilteredRows lists every store whose name matches the filter,
// at depth 0, with no group rows. Filter mode is meant to be a flat
// finder, not a hierarchical browser.
func (s *Stores) flattenedFilteredRows() []Row {
	q := strings.ToLower(s.filter)
	out := make([]Row, 0, len(s.names))
	for _, name := range s.names {
		if !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		d := s.data[name]
		out = append(out, Row{
			Name:    name,
			Display: name,
			Summary: d.summary,
			State:   d.state,
			Fresh:   d.fresh,
		})
	}
	return out
}

// tnode is an ephemeral tree built from the sorted name list to drive
// rendering. It is not retained between rebuilds.
type tnode struct {
	seg       string
	path      string
	storeName string // non-empty when this node is also a leaf store
	store     *storeData
	children  []*tnode
}

func (s *Stores) treeRows() []Row {
	root := &tnode{}
	for _, name := range s.names {
		parts := strings.Split(name, "/")
		cur := root
		for i, p := range parts {
			child := findChild(cur, p)
			if child == nil {
				fullPath := p
				if cur.path != "" {
					fullPath = cur.path + "/" + p
				}
				child = &tnode{seg: p, path: fullPath}
				cur.children = append(cur.children, child)
			}
			if i == len(parts)-1 {
				d := s.data[name]
				child.storeName = name
				child.store = &d
			}
			cur = child
		}
	}

	var out []Row
	var walk func(n *tnode, depth int)
	walk = func(n *tnode, depth int) {
		for _, c := range n.children {
			isPureGroup := c.store == nil && len(c.children) > 0
			if isPureGroup {
				state, leaves := aggregateGroup(c)
				expanded := s.expanded[c.path]
				out = append(out, Row{
					Name:            c.path,
					Display:         c.seg,
					Depth:           depth,
					IsGroup:         true,
					Expanded:        expanded,
					DescendantCount: leaves,
					Summary:         plural(leaves, "store", "stores"),
					State:           state,
				})
				if expanded {
					walk(c, depth+1)
				}
				continue
			}
			// Leaf store row, or store-with-children (rare collision).
			if c.store != nil {
				display := c.seg
				if depth == 0 {
					display = c.storeName
				}
				out = append(out, Row{
					Name:    c.storeName,
					Display: display,
					Depth:   depth,
					Summary: c.store.summary,
					State:   c.store.state,
					Fresh:   c.store.fresh,
				})
			}
			if len(c.children) > 0 {
				// Collision case: show descendants flat under the leaf.
				walk(c, depth+1)
			}
		}
	}
	walk(root, 0)
	return out
}

func findChild(n *tnode, seg string) *tnode {
	for _, c := range n.children {
		if c.seg == seg {
			return c
		}
	}
	return nil
}

func aggregateGroup(n *tnode) (State, int) {
	var states []State
	var walk func(*tnode)
	walk = func(t *tnode) {
		if t.store != nil {
			states = append(states, t.store.state)
		}
		for _, c := range t.children {
			walk(c)
		}
	}
	walk(n)
	return aggregateStates(states), len(states)
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
